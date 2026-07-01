# Release intros

Each `vX.Y.Z.md` here holds the **friendly intro paragraph** for that release —
the warm, human summary that opens the GitHub release body, above the changelog
sections.

`release.sh` picks up the intro in this priority order:

1. `RELEASE_INTRO` environment variable
2. `release-notes/v<version>.md` (this directory)
3. A generic fallback line (avoid for human-facing releases)

Keep it to 2–4 sentences of plain language: what users get, why it matters, and
any headline features or important fixes. Upbeat but honest; markdown and emoji
are welcome. See `RELEASES.md` → "Friendly intros" for details.
