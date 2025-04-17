#!/usr/bin/env bash

# END THE INSTALLATION IF ANYTHING GIVES US A NON-ZERO EXIT CODE
set -e

# CHECK IF USER HAS SUDO RIGHTS
if [[ ${EUID} != 0 ]]; then
	echo "Please run this script as root!" && exit 1
fi

# REMOVE OLD SYSCHECKS INSTALLATION
rm -rf /opt/syschecks

# CREATE A NEW SYSCHECKS FOLDER
mkdir -p /opt/syschecks

# DOWNLOAD ALL REQUIRED FILES
cd /opt/syschecks || (echo "Fatal: could not cd into /opt/syschecks" && exit 1)
wget https://gitlab.gateway-it.com/yaroslav/syschecks/-/raw/main/bin/syschecks
wget https://gitlab.gateway-it.com/yaroslav/syschecks/-/raw/main/package.lock.json

# REMOVE DEPRECATED CRON JOBS
rm -f /etc/cron.d/automatic_system_updates_hold || true
rm -f /etc/cron.d/automatic_security_updates || true
rm -f /etc/cron.d/automatic_system_updates || true
rm -f /etc/cron.d/syschecks || true

# COPY SYSCHECKS BINARY TO /BIN
rm -f /bin/syschecks || true
cp /opt/syschecks/syschecks /bin/
chown root:root /bin/syschecks
chmod 0755 /bin/syschecks

# SET CORRECT PERMISSIONS
chown -R root:root /opt/syschecks
chmod 0755 /opt/syschecks
chmod 0755 /opt/syschecks/syschecks
chmod 0644 /opt/syschecks/package.lock.json

# ENABLE SYSTEM-WIDE BASH COMPLETION
syschecks completion bash >/etc/bash_completion.d/syschecks
chmod 0644 /etc/bash_completion.d/syschecks

# ENABLE CACHE UPDATES
syschecks cron init
# ENABLE DAILY, AUTOMATED SECURITY UPDATES
syschecks cron updates --security

# CREATE SYSCHECKS CACHE FILE
syschecks updates --cache-create

# SIGNAL TO THE USER THAT THE INSTALLATION IS DONE
echo ""
echo ""
echo " >> The installation is now finished"
echo " >> If you'd like to see a banner below on every login, execute the one-liner below (from a root user):"
echo "    echo '([ -z \"\$PS1\" ] && true) || syschecks banner' >> /etc/profile.d/syschecks_banner.sh && chmod 0755 /etc/profile.d/syschecks_banner.sh"
echo ""
echo ""

# SHOW THE SYSCHECKS BANNER
syschecks banner
