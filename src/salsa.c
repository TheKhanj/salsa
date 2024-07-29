#include <assert.h>
#include <bits/pthreadtypes.h>
#include <ctype.h>
#include <inttypes.h>
#include <pthread.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

bool help = false;

volatile bool stop_healthcheck = false;

typedef char *address_t;

void address_host(address_t addr, char *host) {
	int i = 0;
	while (*addr != ':') {
		host[i] = addr[i];
		i++;
	}
	host[i] = '\0';

	if (i == 0) {
		strcpy(host, "0.0.0.0");
		return;
	}
}

uint16_t address_port(address_t addr) {
	while (*addr != ':')
		addr++;
	addr++;
	return atoi(addr);
}

bool address_validate(address_t addr) {
	char *end = addr + strlen(addr);
	while (addr < end && *addr != ':')
		addr++;
	if (addr == end)
		return false;
	addr++;
	while (addr < end) {
		if (!isdigit(*addr))
			return false;
		addr++;
	}
	return true;
}

typedef enum {
	UNABLE_TO_START_PROXY_CHECKER_THREAD = 0,
	UNABLE_TO_FORK_PROCESS,
	UNABLE_TO_WAIT_FOR_CHILD_PROCESS_TO_EXIT,
	UNABLE_TO_WAIT_FOR_PROXY_CHECKER_THREAD_TO_EXIT,
	INVALID_LISTENING_ADDRESS,
	INVALID_SCHEDULER,
	INVALID_BACKEND_ADDRESS,
	MAXIMUM_NUMBER_OF_PROXIES_REACHED,
	NO_BACKEND_PROVIDED,
	NO_LISTENING_ADDRESS_PROVIDED,
	NO_SCOREBOARD_COMMAND_PROVIDED,
	FAILED_CREATING_PROC_DIR,
	FAILED_WRITING_FILE
} err_t;

char *err_msg(err_t err) {
	switch (err) {
	case UNABLE_TO_START_PROXY_CHECKER_THREAD:
		return "unable to start proxy checker thread";
	case UNABLE_TO_FORK_PROCESS:
		return "unable to fork process";
	case UNABLE_TO_WAIT_FOR_CHILD_PROCESS_TO_EXIT:
		return "unable to wait for child process";
	case UNABLE_TO_WAIT_FOR_PROXY_CHECKER_THREAD_TO_EXIT:
		return "unable to wait for proxy checker thread to exit";
	case INVALID_LISTENING_ADDRESS:
		return "invalid listening address";
	case INVALID_SCHEDULER:
		return "invalid scheduler";
	case INVALID_BACKEND_ADDRESS:
		return "invalid backend address";
	case MAXIMUM_NUMBER_OF_PROXIES_REACHED:
		return "maximum number of proxies reached";
	case NO_BACKEND_PROVIDED:
		return "not backend provided";
	case NO_LISTENING_ADDRESS_PROVIDED:
		return "no listening address provided";
	case NO_SCOREBOARD_COMMAND_PROVIDED:
		return "no scoreboard command provided";
	case FAILED_CREATING_PROC_DIR:
		return "failed creating proc dir";
	case FAILED_WRITING_FILE:
		return "failed writing file";
	}
}

void crash(err_t err) {
	fprintf(stderr, "%s\n", err_msg(err));
	exit(err);
}

void file_write(char *path, char *start, size_t len) {
	FILE *file = fopen(path, "w");

	if (file == NULL)
		crash(FAILED_WRITING_FILE);

	fwrite(start, sizeof(char), len, file);
	fclose(file);
}

void exec_socat(address_t addr, const char *handler) {
	char inbound[MAX_STR_LEN], outbound[MAX_STR_LEN],
			scheduler_command[MAX_STR_LEN], host[MAX_STR_LEN];

	address_host(addr, host);

	sprintf(inbound, "TCP-LISTEN:%" PRIu16 ",bind:%s,fork,reuseaddr",
					address_port(addr), host);
	sprintf(scheduler_command, "%s/%s %d", LIB_DIR, handler, getpid());
	sprintf(outbound, "EXEC:%s", scheduler_command);

	char *const cmd[] = {"socat", inbound, outbound, NULL};
	execv(cmd[0], cmd);
}

void exec_man_page() {
	char *const cmd[] = {"man", "socat", NULL};

	execv(cmd[0], cmd);
}

typedef enum {
	ROUND_ROBIN_SCHEDULER = 0,
	SCOREBOARD_SCHERDULER,
} scheduler_t;

typedef struct {
	int weights[MAX_BACKENDS];
} round_robin_scheduler_t;

void round_robin_scheduler_init(round_robin_scheduler_t *self) {
	for (int i = 0; i < MAX_BACKENDS; i++)
		self->weights[i] = 1;
}

typedef struct {
	char *command;
} scoreboard_scheduler_t;

void scoreboard_scheduler_init(scoreboard_scheduler_t *self) {
	self->command = NULL;
}

typedef struct {
	address_t listening_address;
	int backend_count;
	char *backends[MAX_BACKENDS];

	int interval;
	char *healthcheck_command;

	scheduler_t sched;
	round_robin_scheduler_t round_robin_sched;
	scoreboard_scheduler_t scoreboard_sched;
} master_t;

void master_init(master_t *self) {
	self->listening_address = NULL;
	self->backend_count = 0;

	self->interval = 5;
	self->healthcheck_command = "true";
	self->sched = ROUND_ROBIN_SCHEDULER;
	round_robin_scheduler_init(&self->round_robin_sched);
	scoreboard_scheduler_init(&self->scoreboard_sched);
}

char *master_scheduler(master_t *self) {
	switch (self->sched) {
	case ROUND_ROBIN_SCHEDULER:
		return "round_robin";
	case SCOREBOARD_SCHERDULER:
		return "scoreboard_robin";
	}
}

void master_proc_dir(const master_t *self, char *proc_dir) {
	if (getuid() == 0)
		sprintf(proc_dir, "/var/run/salsa/%d", getpid());
	else
		sprintf(proc_dir, "/run/user/%d/salsa/%d", getuid(), getpid());
}

void master_cleanup(master_t *self) {
	return;
	char proc_dir[MAX_STR_LEN];
	master_proc_dir(self, proc_dir);

	char cmd[MAX_STR_LEN];
	sprintf(cmd, "rm -r %s", proc_dir);
	system(cmd);
}

void *healthcheck_loop(void *arg) {
	const master_t *master = arg;

	while (!stop_healthcheck) {
		for (int i = 0; i < master->backend_count; i++) {
			char *backend = master->backends[i];

			pid_t pid = fork();
			if (pid < 0) {
				fprintf(stderr, "%s\n", err_msg(UNABLE_TO_FORK_PROCESS));
				continue;
			}

			if (pid == 0) {
				char *const cmd[] = {master->healthcheck_command, backend, NULL};
				execvp(cmd[0], cmd);
			} else {
				int status;
				waitpid(pid, &status, 0);

				char file[MAX_STR_LEN], proc_dir[MAX_STR_LEN];
				master_proc_dir(master, proc_dir);

				sprintf(file, "%s/backends/%s/health", proc_dir, backend);
				bool is_up = status == 0;
				if (is_up && !was_up)
					fprintf(stderr, "backend %s is back up", backend);
				if (!is_up && was_up)
					fprintf(stderr, "backend %s is down", backend);
				file_write(file, is_up ? "1" : "0", 1);
			}
		}

		sleep(master->interval);
	}

	return NULL;
}

void init_proc(master_t *master) {
	char proc_dir[MAX_STR_LEN];
	master_proc_dir(master, proc_dir);

	char cmd[MAX_STR_LEN];
	sprintf(cmd, "mkdir -p %s", proc_dir);
	system(cmd);

	char *sched = master_scheduler(master);
	char *sched_dir = proc_dir;

	char file[MAX_STR_LEN];

	char dir[MAX_STR_LEN];
	sprintf(dir, "%s/backends", proc_dir);
	mkdir(dir, 0755);
	for (int i = 0; i < master->backend_count; i++) {
		char *backend = master->backends[i];
		sprintf(dir, "%s/backends/%s", proc_dir, backend);
		mkdir(dir, 0755);
	}

	// TODO: create proxies dir

	// TODO: move to thread
	// switch (master->sched) {
	// case ROUND_ROBIN_SCHEDULER:
	// 	sprintf(file, "%s/proxies", sched_dir);
	// 	// TODO: fix this shit
	// 	char w[MAX_STR_LEN];
	// 	int len = 0;
	// 	for (int j = 0; j < master->backend_count; j++) {
	// 		char s[MAX_STR_LEN];
	// 		sprintf(s, "%d%c", master->round_robin_sched.weights[j],
	// 						j == master->backend_count - 1 ? '\0' : ',');
	// 		sprintf(w + len, "%s", s);
	// 		len += strlen(s);
	// 	}
	// 	file_write(file, w, len);
	// 	break;
	// case SCOREBOARD_SCHERDULER:
	// 	sprintf(file, "%s/command", sched_dir);
	// 	file_write(file, master->scoreboard_sched.command,
	// 						 strlen(master->scoreboard_sched.command));
	// 	break;
	// }

	sprintf(file, "%s/scheduler", proc_dir);
	file_write(file, sched, strlen(sched));
}

void parse_args(int argc, char **argv, master_t *master) {
	int i = 1;
	while (i < argc) {
		char *s = argv[i];
		if (strcmp(s, "-l") == 0 || strcmp(s, "--listen") == 0) {
			assert(i + 1 < argc);
			master->listening_address = argv[i + 1];
			if (!address_validate(master->listening_address))
				crash(INVALID_LISTENING_ADDRESS);

			i += 2;
		} else if (strcmp(s, "-c") == 0 || strcmp(s, "--check") == 0) {
			assert(i + 1 < argc);
			master->healthcheck_command = argv[i + 1];

			i += 2;
		} else if (strcmp(s, "-s") == 0 || strcmp(s, "--scheduler") == 0) {
			assert(i + 1 < argc);
			char *sched = argv[i + 1];
			if (strcmp(sched, "round-robin") == 0)
				master->sched = ROUND_ROBIN_SCHEDULER;
			else if (strcmp(sched, "scoreboard") == 0)
				master->sched = SCOREBOARD_SCHERDULER;
			else
				crash(INVALID_SCHEDULER);

			i += 2;
		} else if (strcmp(s, "--round-robin-weights") == 0) {
			assert(i + 1 < argc);
			char *w = argv[i + 1];
			char *e = w + strlen(w);
			int j = 0;
			while (w < e) {
				int weight = atoi(w);
				master->round_robin_sched.weights[j++] = weight;

				if (w < e && *w != ',')
					w++;
				w++;
			}

			i += 2;
		} else if (strcmp(s, "--scoreboard-command") == 0) {
			assert(i + 1 < argc);
			master->scoreboard_sched.command = argv[i + 1];

			i += 2;
		} else if (strcmp(s, "-h") == 0 || strcmp(s, "--help") == 0) {
			help = true;

			i++;
		} else {
			if (!address_validate(s))
				crash(INVALID_BACKEND_ADDRESS);
			if (master->backend_count == MAX_BACKENDS)
				crash(MAXIMUM_NUMBER_OF_PROXIES_REACHED);
			master->backends[master->backend_count++] = s;

			i++;
		}
	}

	if (master->backend_count == 0)
		crash(NO_BACKEND_PROVIDED);
	if (master->listening_address == NULL)
		crash(NO_LISTENING_ADDRESS_PROVIDED);
	if (master->sched == SCOREBOARD_SCHERDULER &&
			master->scoreboard_sched.command == NULL)
		crash(NO_SCOREBOARD_COMMAND_PROVIDED);
}

int main(int argc, char **argv) {
	master_t master;
	master_init(&master);

	pthread_t thread;

	parse_args(argc, argv, &master);

	if (help) {
		exec_man_page();
		return 0;
	}

	init_proc(&master);

	if (pthread_create(&thread, NULL, healthcheck_loop, &master) != 0)
		crash(UNABLE_TO_START_PROXY_CHECKER_THREAD);

	pid_t pid = fork();
	if (pid < 0)
		crash(UNABLE_TO_FORK_PROCESS);
	if (pid == 0) {
		char handler[MAX_STR_LEN];
		sprintf(handler, "%s_handler", master_scheduler(&master));
		exec_socat(master.listening_address, handler);
	} else {
		int child_exit_code;

		if (waitpid(pid, &child_exit_code, 0) == -1)
			crash(UNABLE_TO_WAIT_FOR_CHILD_PROCESS_TO_EXIT);

		stop_healthcheck = true;
		if (pthread_join(thread, NULL) != 0)
			crash(UNABLE_TO_WAIT_FOR_PROXY_CHECKER_THREAD_TO_EXIT);

		master_cleanup(&master);

		return child_exit_code;
	}
}
