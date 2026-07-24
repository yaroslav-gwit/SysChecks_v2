package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

var (
	userinfoJSON       bool
	userinfoJSONPretty bool
	userinfoAllUsers   bool

	userinfoCmd = &cobra.Command{
		Use: "users",
		// The old name stays as an alias so existing scripts and Zabbix items keep working.
		Aliases: []string{"userinfo"},
		Short:   "List real system users and who is logged in",
		Long: `List real system users: whether they are currently logged in, password lock
state, and last login source.

"Logged in" reflects live sessions only. Whether the account itself is usable is reported
by the Password column (locked / unlocked).`,
		Run: func(cmd *cobra.Command, args []string) { runUsersList() },
	}

	usersListCmd = &cobra.Command{
		Use:   "list",
		Short: "List real system users and who is logged in",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { runUsersList() },
	}
)

func runUsersList() {
	users := collectUserInfo(userinfoAllUsers)

	if format := resolveOutput(outputText, userinfoJSON, userinfoJSONPretty); format != outputText {
		emitJSON(users, format)
		return
	}

	printUserInfoTable(users)
}

type userInfoRecord struct {
	Username string `json:"username"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	FullName string `json:"full_name,omitempty"`
	Home     string `json:"home"`
	Shell    string `json:"shell"`
	// LoggedIn describes session state, not account state. It was previously called
	// "active", which operators read as "the account is enabled".
	LoggedIn       bool   `json:"logged_in"`
	LoginSessions  int    `json:"login_sessions"`
	LoginSources   string `json:"login_sources,omitempty"`
	PasswordStatus string `json:"password_status"`
	PasswordLocked *bool  `json:"password_locked,omitempty"`
	LastLogin      string `json:"last_login"`
	LastTTY        string `json:"last_tty,omitempty"`
	LastSource     string `json:"last_source"`
	LastHost       string `json:"last_host,omitempty"`
}

type passwdEntry struct {
	username string
	uid      int
	gid      int
	fullName string
	home     string
	shell    string
}

type loginEntry struct {
	when   string
	tty    string
	host   string
	source string
}

type loginDefRange struct {
	uidMin int
	uidMax int
}

func collectUserInfo(includeAll bool) []userInfoRecord {
	passwdEntries := readPasswdEntries("/etc/passwd")
	uidRange := readLoginDefRange("/etc/login.defs")
	shadowStatuses := readShadowStatuses("/etc/shadow")
	loginSessions := readLoginSessions()

	records := make([]userInfoRecord, 0, len(passwdEntries))
	for _, entry := range passwdEntries {
		if !includeAll && !isRealUser(entry, uidRange) {
			continue
		}

		lastLogin := lastLoginForUser(entry.username)
		passwordStatus, passwordLocked := passwordStatusForUser(entry.username, shadowStatuses)
		loginSources := loginSessions[entry.username]

		records = append(records, userInfoRecord{
			Username:       entry.username,
			UID:            entry.uid,
			GID:            entry.gid,
			FullName:       entry.fullName,
			Home:           entry.home,
			Shell:          entry.shell,
			LoggedIn:       len(loginSources) > 0,
			LoginSessions:  len(loginSources),
			LoginSources:   strings.Join(loginSources, ", "),
			PasswordStatus: passwordStatus,
			PasswordLocked: passwordLocked,
			LastLogin:      lastLogin.when,
			LastTTY:        lastLogin.tty,
			LastSource:     lastLogin.source,
			LastHost:       lastLogin.host,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].UID == records[j].UID {
			return records[i].Username < records[j].Username
		}
		return records[i].UID < records[j].UID
	})

	return records
}

func readPasswdEntries(path string) []passwdEntry {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Warning: could not read %s: %v", path, err)
		return nil
	}
	defer file.Close()

	var entries []passwdEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}

		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		gid, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}

		entries = append(entries, passwdEntry{
			username: parts[0],
			uid:      uid,
			gid:      gid,
			fullName: strings.Split(parts[4], ",")[0],
			home:     parts[5],
			shell:    parts[6],
		})
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Warning: could not scan %s: %v", path, err)
	}

	return entries
}

func readLoginDefRange(path string) loginDefRange {
	result := loginDefRange{uidMin: 1000, uidMax: 60000}

	file, err := os.Open(path)
	if err != nil {
		return result
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		value, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "UID_MIN":
			result.uidMin = value
		case "UID_MAX":
			result.uidMax = value
		}
	}

	return result
}

func isRealUser(entry passwdEntry, uidRange loginDefRange) bool {
	if entry.uid == 0 {
		return true
	}
	if entry.uid < uidRange.uidMin || entry.uid > uidRange.uidMax {
		return false
	}
	return isLoginShell(entry.shell)
}

func isLoginShell(shell string) bool {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return false
	}

	base := shell
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}

	switch base {
	case "false", "nologin", "sync", "shutdown", "halt":
		return false
	default:
		return true
	}
}

func readShadowStatuses(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	statuses := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		statuses[parts[0]] = parts[1]
	}

	return statuses
}

func passwordStatusForUser(username string, shadowStatuses map[string]string) (string, *bool) {
	if shadowStatuses == nil {
		return "unknown (requires root)", nil
	}

	hash, ok := shadowStatuses[username]
	if !ok {
		return "unknown", nil
	}

	locked := false
	switch {
	case hash == "":
		return "empty password", &locked
	case strings.HasPrefix(hash, "!") || strings.HasPrefix(hash, "*"):
		locked = true
		return "locked", &locked
	default:
		return "unlocked", &locked
	}
}

func readLoginSessions() map[string][]string {
	out, err := exec.Command("who").Output()
	if err != nil {
		return map[string][]string{}
	}
	return parseWhoOutput(string(out))
}

func parseWhoOutput(output string) map[string][]string {
	sessions := make(map[string][]string)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		username := fields[0]
		tty := fields[1]
		host := ""
		if len(fields) >= 3 {
			lastField := fields[len(fields)-1]
			if strings.HasPrefix(lastField, "(") && strings.HasSuffix(lastField, ")") {
				host = strings.Trim(lastField, "()")
			}
		}
		source := classifyLoginSource(tty, host)
		session := tty
		if host != "" {
			session += "@" + host
		}
		session += " (" + source + ")"
		sessions[username] = append(sessions[username], session)
	}

	return sessions
}

func lastLoginForUser(username string) loginEntry {
	out, err := exec.Command("last", "-w", "-F", username).Output()
	if err != nil && len(out) == 0 {
		return loginEntry{when: "never", source: "never"}
	}
	return parseLastLoginOutput(username, string(out))
}

func parseLastLoginOutput(username string, output string) loginEntry {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "wtmp begins") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[0] != username {
			continue
		}

		tty := fields[1]
		host := ""
		dateStart := 2
		if !looksLikeWeekday(fields[2]) {
			host = fields[2]
			dateStart = 3
		}
		if len(fields) < dateStart+5 {
			continue
		}

		whenFields := fields[dateStart : dateStart+5]
		when := strings.Join(whenFields, " ")
		if parsed, err := time.Parse("Mon Jan 2 15:04:05 2006", when); err == nil {
			when = parsed.Format("2006-01-02 15:04:05")
		}

		return loginEntry{
			when:   when,
			tty:    tty,
			host:   host,
			source: classifyLoginSource(tty, host),
		}
	}

	return loginEntry{when: "never", source: "never"}
}

func looksLikeWeekday(value string) bool {
	switch value {
	case "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun":
		return true
	default:
		return false
	}
}

func classifyLoginSource(tty string, host string) string {
	tty = strings.TrimSpace(tty)
	host = strings.TrimSpace(strings.Trim(host, "()"))

	switch {
	case strings.HasPrefix(host, "tmux("):
		return "tmux"
	case host != "":
		return "ssh"
	case strings.HasPrefix(tty, "tty") || strings.HasPrefix(tty, "console"):
		return "getty/console"
	case strings.HasPrefix(tty, "seat"):
		return "local graphical"
	case strings.HasPrefix(tty, "pts/"):
		return "local terminal"
	default:
		return "unknown"
	}
}

func printUserInfoTable(users []userInfoRecord) {
	// Keep the default table compact enough for a standard 80-column SSH
	// terminal. The complete session and account details remain in JSON output.
	// "Logged in" rather than "Active": operators read "active" as "the account is enabled",
	// which is what the Password column actually reports.
	headers := []string{"User", "UID", "Logged in", "Password", "Last login", "Source"}
	rows := make([][]string, 0, len(users))

	for _, user := range users {
		loggedIn := "no"
		if user.LoggedIn {
			loggedIn = fmt.Sprintf("yes (%d)", user.LoginSessions)
		}

		passwordStatus := user.PasswordStatus
		if strings.HasPrefix(passwordStatus, "unknown") {
			passwordStatus = "unknown"
		}

		lastSource := user.LastSource
		if user.LastHost != "" && user.LastSource == "ssh" {
			lastSource = shortenCell(lastSource+"@"+user.LastHost, 20)
		}

		rows = append(rows, []string{
			user.Username,
			strconv.Itoa(user.UID),
			loggedIn,
			passwordStatus,
			user.LastLogin,
			lastSource,
		})
	}

	printTable(headers, rows)
}

// shortenCell truncates to a column count, not a byte count. Repository diagnoses contain
// em dashes, and slicing bytes would cut a multi-byte rune in half and print a replacement
// character inside the banner.
func shortenCell(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	if maximum <= 3 {
		return string(runes[:maximum])
	}
	return string(runes[:maximum-3]) + "..."
}

func printTable(headers []string, rows [][]string) {
	fprintTable(os.Stdout, headers, rows)
}

// fprintTable sizes and pads by display width, not byte length. A cell containing an emoji
// or any non-ASCII text is more bytes than columns, and padding by bytes would push that
// row's borders out of line with every other row.
func fprintTable(out io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = runewidth.StringWidth(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if width := runewidth.StringWidth(cell); width > widths[i] {
				widths[i] = width
			}
		}
	}

	printBorder(out, "╭", "┬", "╮", widths)
	printRow(out, headers, widths)
	printBorder(out, "├", "┼", "┤", widths)
	for _, row := range rows {
		printRow(out, row, widths)
	}
	printBorder(out, "╰", "┴", "╯", widths)
}

func printBorder(out io.Writer, left string, middle string, right string, widths []int) {
	fmt.Fprint(out, left)
	for i, width := range widths {
		fmt.Fprint(out, strings.Repeat("─", width+2))
		if i == len(widths)-1 {
			fmt.Fprint(out, right)
		} else {
			fmt.Fprint(out, middle)
		}
	}
	fmt.Fprintln(out)
}

func printRow(out io.Writer, cells []string, widths []int) {
	fmt.Fprint(out, "│")
	for i, cell := range cells {
		// Pad manually: %-*s counts bytes, so it under-pads any cell holding an emoji.
		padding := widths[i] - runewidth.StringWidth(cell)
		if padding < 0 {
			padding = 0
		}
		fmt.Fprint(out, " "+cell+strings.Repeat(" ", padding)+" │")
	}
	fmt.Fprintln(out)
}
