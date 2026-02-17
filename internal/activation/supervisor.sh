#!/bin/sh

# The activation directory may not exist at this point,
# and creating the lockfile would fail otherwise.
mkdir -p /run/nixos

# Concurrent supervisors are prohibited.
# Otherwise, multiple rollbacks and switch-to-configuration
# invocations could have a chance of occurring and makes
# error output super confusing.
LOCKFILE=/run/nixos/activation-supervisor.lock
SWITCH_LOCKFILE=/run/nixos/switch-to-configuration.lock

# The appearance of this path signals to the command invoker
# that the initial switch-to-configuration has completed
# without any errors.
SWITCH_SUCCESS_PATH=/run/nixos/switch-success

log() {
	echo "$@" >&2
}

if [ "$ACK_TIMEOUT" = "" ] || ! [ "$ACK_TIMEOUT" -gt 0 ] 2>/dev/null; then
	log "ACK_TIMEOUT must be set to a positive integer"
	exit 1
fi

# Pass in -v to switch-to-configuration runs if this is set.
verbose_flag=
if [ "$VERBOSE" != "" ]; then
	verbose_flag="-v"
fi

# Remove stale entries for the file signals,
# and make sure they are removed if exiting or interrupted.
rm -f "$ACK_TRIGGER_PATH" "$SWITCH_SUCCESS_PATH"
trap 'rm -f "$ACK_TRIGGER_PATH" "$SWITCH_SUCCESS_PATH"' EXIT INT TERM HUP QUIT

# Use fd 7 to obtain the process lock.
exec 7>"$LOCKFILE" || {
	log "failed to map fd 7 to $LOCKFILE"
	exit 1
}

flock -n 7 || {
	log "failed to lock $LOCKFILE; is another activation process running?"
	exit 1
}

# If a specialisation is passed by the caller, then use that
# switch-to-configuration script instead.
if [ "$SPECIALISATION" = "" ]; then
	switchToConfigurationScript="$TOPLEVEL/bin/switch-to-configuration"
else
	switchToConfigurationScript="$TOPLEVEL/specialisation/$SPECIALISATION/bin/switch-to-configuration"
fi

# Save the previous generation's path now in case a rollback
# needs to be run.
prevToplevel=$(readlink /run/current-system)

rollback() {
	# $ROLLBACK_PROFILE_ON_FAILURE will not be set when switching
	# from a system that does not exist in the generation list,
	# such as when switching from a generation built with
	# `nixos apply --no-boot` or `nixos-rebuild test` to another
	# one.
	if [ "$ROLLBACK_PROFILE_ON_FAILURE" != "" ]; then
		nix-env -p "$PROFILE" --rollback "$verbose_flag"
	fi

	# If a previous specialisation is detected by the caller, then use it.
	if [ "$PREVIOUS_SPECIALISATION" = "" ]; then
		prevSwitchToConfigurationScript="$prevToplevel/bin/switch-to-configuration"
	else
		prevSwitchToConfigurationScript="$prevToplevel/specialisation/$PREVIOUS_SPECIALISATION/bin/switch-to-configuration"
	fi

	"$prevSwitchToConfigurationScript" "$ACTION" "$verbose_flag"

	exit 1
}

# Run the switch, and immediately rollback if it fails.
if ! "$switchToConfigurationScript" "$ACTION" "$verbose_flag"; then
	rollback
fi

# Obtain a switch-to-configuration lock now on fd 8, to make sure
# no standalone switch-to-configuration processes acquire the lock
exec 8>"$SWITCH_LOCKFILE" || {
	log "failed to map fd 8 to $SWITCH_LOCKFILE"
	exit 1
}

flock -n 8 || {
	log "failed to lock $SWITCH_LOCKFILE; another activation process may have stolen the limelight"
	log "running switch-to-configuration during the acknowledgement watchdog process is strongly not recommended"
	log "not attempting to proceed with rollback detection"
	exit 1
}

# Signal to callers that switch-to-configuration has been run, and
# that we are now waiting for an acknowledgement of it.
touch "$SWITCH_SUCCESS_PATH"

log "waiting for acknowledgement"

# Poll for the file once a second for $ACK_TIMEOUT seconds; this is a POSIX sh
# substitute for `inotifywait` to avoid introducing additional dependencies.
i=0
while [ "$i" -lt "$ACK_TIMEOUT" ]; do
	if [ -f "$ACK_TRIGGER_PATH" ]; then
		log "acknowledgement received; activation is now complete"
		exit 0
	fi

	sleep 1
	i=$((i + 1))
done

if [ -f "$ACK_TRIGGER_PATH" ]; then
	log "acknowledgement received; activation is now complete"
	exit 0
fi

flock -un 8

log "no acknowledgement was received from invoking machine; starting rollback"
rollback
