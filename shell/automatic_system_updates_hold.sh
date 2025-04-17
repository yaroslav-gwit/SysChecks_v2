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

	# DOCKER AND NVIDIA UPDATES ARE LOCKED TO AVOID ANY PROCESSING/CONTAINER ISSUES
	apt-mark hold docker*
	apt-mark hold containerd*
	apt-mark hold cuda-drivers-*
	apt-mark hold libnvidia-*
	apt-mark hold nvidia-*
	apt-mark hold xserver-xorg-video-nvidia-*

	# UPDATE THE SYSTEM
	export DEBIAN_FRONTEND=noninteractive
	export APT_LISTCHANGES_FRONTEND=none
	apt-get update 2>&1 | tee -a ${LOG_FILE}
	apt-get dist-upgrade -f -u -y --allow-downgrades --allow-remove-essential --allow-change-held-packages -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" 2>&1 | tee -a ${LOG_FILE}

	# DOCKER AND NVIDIA UPDATES ARE ENABLED AGAIN AT THE END OF THE PROCESS
	apt-mark unhold docker*
	apt-mark unhold containerd*
	apt-mark unhold cuda-drivers-*
	apt-mark unhold libnvidia-*
	apt-mark unhold nvidia-*
	apt-mark unhold xserver-xorg-video-nvidia-*

elif [[ $(grep "ID=" /etc/os-release | grep -c "centos") -gt 0 ]]; then
	if [[ -f ${LOG_FILE} ]]; then echo "" >>${LOG_FILE} && echo "" >>${LOG_FILE}; fi
	echo "#_ $(date) _#" >>${LOG_FILE}

	# DOCKER UPDATES ARE LOCKED TO AVOID CONTAINER FAILURES
	yum versionlock docker* 2>&1 | tee -a ${LOG_FILE}
	yum versionlock docker-* 2>&1 | tee -a ${LOG_FILE}
	yum versionlock containerd* 2>&1 | tee -a ${LOG_FILE}

	# UPDATE THE SYSTEM
	yum -y update 2>&1 | tee -a ${LOG_FILE}

	# DOCKER UPDATES ARE ENABLED AGAIN AT THE END OF THE PROCESS
	yum versionlock delete docker* 2>&1 | tee -a ${LOG_FILE}
	yum versionlock delete docker-* 2>&1 | tee -a ${LOG_FILE}
	yum versionlock delete containerd* 2>&1 | tee -a ${LOG_FILE}

	# SET THE LATEST KERNEL TO BOOT AUTOMATICALLY
	grub2-set-default 0
	grub2-mkconfig -o /boot/grub2/grub.cfg

elif [[ $(grep "^ID=" /etc/os-release | grep -c 'almalinux\|"ol"\|"rocky"') -gt 0 ]]; then
	if [[ -f ${LOG_FILE} ]]; then echo "" >>${LOG_FILE} && echo "" >>${LOG_FILE}; fi
	echo "#_ $(date) _#" >>${LOG_FILE}

	# DOCKER UPDATES ARE LOCKED TO AVOID CONTAINER FAILURES
	dnf versionlock docker* 2>&1 | tee -a ${LOG_FILE}
	dnf versionlock docker-* 2>&1 | tee -a ${LOG_FILE}
	dnf versionlock containerd* 2>&1 | tee -a ${LOG_FILE}

	# UPDATE THE SYSTEM
	dnf -y update 2>&1 | tee -a ${LOG_FILE}

	# DOCKER UPDATES ARE ENABLED AGAIN AT THE END OF THE PROCESS
	dnf versionlock delete docker* 2>&1 | tee -a ${LOG_FILE}
	dnf versionlock delete docker-* 2>&1 | tee -a ${LOG_FILE}
	dnf versionlock delete containerd* 2>&1 | tee -a ${LOG_FILE}

	# SET THE LATEST KERNEL TO BOOT AUTOMATICALLY
	grub2-set-default 0
	grub2-mkconfig -o /boot/grub2/grub.cfg

else
	echo "Sorry your OS is not yet supported!" && exit 1
	exit 1

fi

syschecks updates --cache-create
syschecks banner
