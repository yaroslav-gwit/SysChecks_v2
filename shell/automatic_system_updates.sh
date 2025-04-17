#!/usr/bin/env bash

# CHECK IF USER HAS SUDO RIGHTS
if [[ ${EUID} != 0 ]]; then
	echo "Please run this script as root!" && exit 1
fi

# SET THE LOG CONFIG LOCATION
LOG_FILE=/var/log/automatic_system_updates.log

# DETECT THE OS TYPE AND START THE UPGRADE
if [[ $(grep "ID=" /etc/os-release | grep -c "ubuntu\|debian\|pop") -gt 0 ]]; then
	if [[ -f ${LOG_FILE} ]]; then echo "" >>${LOG_FILE} && echo "" >>${LOG_FILE}; fi
	echo "#_ $(date) _#" >>${LOG_FILE}
	export DEBIAN_FRONTEND=noninteractive
	export APT_LISTCHANGES_FRONTEND=none
	apt-get update 2>&1 | tee -a ${LOG_FILE}
	apt-get dist-upgrade -f -u -y --allow-downgrades --allow-remove-essential --allow-change-held-packages -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" 2>&1 | tee -a ${LOG_FILE}

elif [[ $(grep "ID=" /etc/os-release | grep -c "centos") -gt 0 ]]; then
	if [[ -f ${LOG_FILE} ]]; then echo "" >>${LOG_FILE} && echo "" >>${LOG_FILE}; fi
	echo "#_ $(date) _#" >>${LOG_FILE}
	yum -y update 2>&1 | tee -a ${LOG_FILE}
	# SET THE LATEST KERNEL TO BOOT AUTOMATICALLY
	grub2-set-default 0
	grub2-mkconfig -o /boot/grub2/grub.cfg

elif [[ $(grep "^ID=" /etc/os-release | grep -c 'almalinux\|"ol"\|"rocky"') -gt 0 ]]; then
	if [[ -f ${LOG_FILE} ]]; then echo "" >>${LOG_FILE} && echo "" >>${LOG_FILE}; fi
	echo "#_ $(date) _#" >>${LOG_FILE}
	dnf -y update 2>&1 | tee -a ${LOG_FILE}
	# SET THE LATEST KERNEL TO BOOT AUTOMATICALLY
	grub2-set-default 0
	grub2-mkconfig -o /boot/grub2/grub.cfg

else
	echo "Sorry your OS is not yet supported!" && exit 1
	exit 1

fi

syschecks updates --cache-create
syschecks banner
