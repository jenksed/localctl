package main

func infraExact(id, title, category, difficulty, description, prompt, expected string) exercise {
	return exercise{ID: id, Title: title, Category: category, Difficulty: difficulty, Description: description, Prompt: prompt, Evaluation: evaluationSpec{Kind: evaluationExact, Expected: expected}}
}

func infraContains(id, title, category, difficulty, description, prompt string, expected ...string) exercise {
	return exercise{ID: id, Title: title, Category: category, Difficulty: difficulty, Description: description, Prompt: prompt, Evaluation: evaluationSpec{Kind: evaluationContainsAll, Contains: expected}}
}

func infraManual(id, title, category, difficulty, description, prompt string) exercise {
	return exercise{ID: id, Title: title, Category: category, Difficulty: difficulty, Description: description, Prompt: prompt, Evaluation: evaluationSpec{Kind: evaluationManual}}
}

func linuxInvestigationExerciseCatalog() []exercise {
	return []exercise{
		infraExact("linux-exit-127-v1", "Command not found", "linux", "easy", "Tests shell exit-status interpretation.", "A POSIX shell returns exit status 127. Reply exactly COMMAND_NOT_FOUND or PERMISSION_DENIED.", "COMMAND_NOT_FOUND"),
		infraExact("linux-exit-126-v1", "Command cannot execute", "linux", "easy", "Tests distinction between missing and non-executable commands.", "A shell finds a command but cannot execute it and returns 126. Reply exactly NOT_EXECUTABLE or NOT_FOUND.", "NOT_EXECUTABLE"),
		infraExact("linux-sigkill-exit-v1", "SIGKILL status", "linux", "medium", "Tests conventional shell signal-exit encoding.", "A process is terminated by SIGKILL (signal 9). In common shells, what numeric status is reported as 128+signal? Reply with only the integer.", "137"),
		infraExact("linux-permission-mode-v1", "Unix mode interpretation", "linux", "easy", "Tests basic file mode reasoning.", "Mode 0640 grants the owner read+write, group read, others none. Reply exactly YES or NO.", "YES"),
		infraExact("linux-owner-write-v1", "Owner write permission", "linux", "easy", "Tests one permission-bit question without prose.", "A regular file has mode 0444. Can its owner write it based on mode bits alone? Reply exactly YES or NO.", "NO"),
		infraExact("linux-symlink-v1", "Symbolic link identity", "linux", "easy", "Tests distinction between link path and target.", "`ln -s /srv/releases/v2 /srv/current` creates /srv/current as what? Reply exactly SYMLINK or COPY.", "SYMLINK"),
		infraExact("linux-hardlink-v1", "Hard-link inode", "linux", "medium", "Tests inode identity for hard links.", "Two hard links to the same regular file on one filesystem normally reference the same inode. Reply exactly YES or NO.", "YES"),
		infraExact("linux-disk-full-v1", "Disk full classification", "linux", "easy", "Tests interpretation of ENOSPC-like behavior.", "An application fails writes with 'No space left on device'. Reply exactly CAPACITY or PERMISSION.", "CAPACITY"),
		infraExact("linux-inodes-full-v1", "Inode exhaustion", "linux", "medium", "Tests recognition that free bytes do not imply files can be created.", "`df -h` shows free space but creating new files returns ENOSPC; `df -i` shows 100% inode use. Reply exactly INODES.", "INODES"),
		infraExact("linux-load-vs-cpu-v1", "Load average meaning", "linux", "medium", "Tests avoiding the common 'load equals CPU percent' mistake.", "Does Linux load average directly equal CPU utilization percentage? Reply exactly YES or NO.", "NO"),
		infraExact("linux-zombie-v1", "Zombie process", "linux", "medium", "Tests recognition of exited-but-unreaped processes.", "A process has exited but its parent has not called wait(), and ps shows state Z. Reply exactly ZOMBIE.", "ZOMBIE"),
		infraExact("linux-orphan-v1", "Orphan process", "linux", "medium", "Tests distinction between orphan and zombie.", "A living child continues after its parent exits and becomes adopted by PID 1 or a subreaper. Reply exactly ORPHAN.", "ORPHAN"),
		infraExact("linux-sigterm-v1", "SIGTERM semantics", "linux", "easy", "Tests graceful-signal interpretation.", "Does SIGTERM give a process an opportunity to handle the signal and clean up? Reply exactly YES or NO.", "YES"),
		infraExact("linux-listen-v1", "Listening socket", "linux", "easy", "Tests what LISTEN proves.", "`ss -ltn` shows 127.0.0.1:8080 in LISTEN. Does this alone prove the HTTP health endpoint returns 200? Reply exactly YES or NO.", "NO"),
		infraExact("linux-loopback-v1", "Loopback binding", "linux", "easy", "Tests reachability implications of 127.0.0.1 binding.", "A service binds only 127.0.0.1. Is it normally reachable directly from another machine on the LAN? Reply exactly YES or NO.", "NO"),
		infraExact("linux-port-owner-v1", "Port owner command", "linux", "easy", "Tests selecting a practical process/socket investigation tool.", "Choose the better command to identify a process listening on TCP 8080: `lsof -iTCP:8080 -sTCP:LISTEN` or `pwd`. Reply exactly LSOF or PWD.", "LSOF"),
		infraExact("linux-dns-v1", "DNS before HTTP", "linux", "easy", "Tests layering in name-resolution failure.", "curl reports 'Could not resolve host'. Has an HTTP request reached the remote server? Reply exactly YES or NO.", "NO"),
		infraExact("linux-timeout-v1", "Connection timeout", "linux", "medium", "Tests careful distinction between timeout and refusal.", "A TCP connect attempt waits until timeout with no response. Reply exactly TIMEOUT or REFUSED.", "TIMEOUT"),
		infraExact("linux-journal-unit-v1", "systemd unit logs", "linux", "easy", "Tests selecting journalctl for a named service.", "Which command family reads logs for systemd unit api.service? Reply exactly JOURNALCTL or CHMOD.", "JOURNALCTL"),
		infraExact("linux-systemctl-failed-v1", "systemd failed state", "linux", "easy", "Tests service-state interpretation.", "`systemctl is-failed api.service` reports failed. Does that prove the service is healthy? Reply exactly YES or NO.", "NO"),
		infraExact("linux-oom-v1", "OOM kill", "linux", "medium", "Tests recognition of kernel out-of-memory termination.", "Kernel logs say 'Out of memory: Killed process 4242 (worker)'. Reply exactly OOM_KILL.", "OOM_KILL"),
		infraExact("linux-memory-free-v1", "Linux free memory", "linux", "medium", "Tests avoiding the simplistic 'free column means available memory' misconception.", "For modern Linux memory headroom, is the `available` value generally more useful than raw `free` alone? Reply exactly YES or NO.", "YES"),
		infraExact("linux-swap-v1", "Swap activity", "linux", "medium", "Tests interpretation of active swapping as pressure evidence.", "Sustained high swap-in and swap-out activity during latency spikes is evidence of memory pressure. Reply exactly YES or NO.", "YES"),
		infraExact("linux-readonly-fs-v1", "Read-only filesystem", "linux", "easy", "Tests error classification.", "Writes fail with 'Read-only file system'. Reply exactly MOUNT_MODE or FILE_OWNER.", "MOUNT_MODE"),
		infraExact("linux-mount-v1", "Mount boundary", "linux", "medium", "Tests understanding that path appearance does not identify backing filesystem.", "Can two paths under /srv reside on different mounted filesystems? Reply exactly YES or NO.", "YES"),
		infraExact("linux-deleted-open-v1", "Deleted open file", "linux", "medium", "Tests why disk space can remain consumed after unlink.", "A huge log was deleted but a running process still has it open. Can disk blocks remain allocated until the file descriptor closes? Reply exactly YES or NO.", "YES"),
		infraExact("linux-fd-limit-v1", "File descriptor exhaustion", "linux", "medium", "Tests recognition of EMFILE-style limits.", "A process repeatedly gets 'Too many open files'. Reply exactly FD_LIMIT.", "FD_LIMIT"),
		infraExact("linux-env-scope-v1", "Environment inheritance", "linux", "medium", "Tests process environment inheritance.", "A parent exports FOO=bar before starting a child. The child normally inherits FOO unless modified. Reply exactly YES or NO.", "YES"),
		infraExact("linux-path-order-v1", "PATH resolution", "linux", "easy", "Tests executable resolution order.", "PATH is /opt/custom/bin:/usr/bin and both contain tool. Which copy is found first? Reply exactly /opt/custom/bin/tool.", "/opt/custom/bin/tool"),
		infraExact("linux-log-rotation-v1", "Log rotation handle", "linux", "medium", "Tests operational behavior when an app holds an old file descriptor.", "A logfile is renamed during rotation but the process keeps writing to its already-open descriptor. Can it continue writing to the renamed inode? Reply exactly YES or NO.", "YES"),
		infraContains("linux-log-investigation-v1", "Linux log investigation", "linux", "hard", "Human-readable investigation task emphasizing evidence before cause.", "Logs show: 10:01 worker started; 10:05 requests slow; 10:05 kernel reports memory pressure; 10:06 OOM kills worker; 10:07 supervisor restarts it. In one short sentence, identify the strongest directly observed failure and the relevant subsystem.", "OOM", "memory"),
		infraManual("linux-service-triage-v1", "Linux service triage", "linux", "hard", "Tests practical investigation sequencing without jumping to redesign.", "A local service that worked yesterday now returns connection refused. Give the first five Linux checks in order, and for each say what observation would change your next step. Maximum 170 words."),
		infraManual("linux-disk-triage-v1", "Linux disk incident", "linux", "hard", "Tests investigation across bytes, inodes, mounts, and deleted-open files.", "An application reports ENOSPC but the operator believes plenty of disk is free. Give a compact investigation plan covering the major distinct Linux causes. Maximum 160 words."),
		infraManual("linux-performance-triage-v1", "Linux latency triage", "linux", "hard", "Tests practical reasoning across CPU, memory, I/O, and process state.", "A Linux API has intermittent 5-second latency spikes. Give an evidence-first triage plan that distinguishes CPU saturation, memory pressure, disk I/O, and downstream waits. Maximum 180 words."),
	}
}
