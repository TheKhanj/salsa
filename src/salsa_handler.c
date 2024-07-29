#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define MAX_PROXIES 1024

char *proxies[MAX_PROXIES];
int num_proxies = 0;

int get_new_proxy(const char *proc_dir) {
	char curr_path[256];
	snprintf(curr_path, sizeof(curr_path), "%s/current", proc_dir);

	FILE *fp = fopen(curr_path, "r+");
	if (fp == NULL) {
		fp = fopen(curr_path, "w+");
		fprintf(fp, "0");
		rewind(fp);
	}

	int proxy_id;
	fscanf(fp, "%d", &proxy_id);
	proxy_id = (proxy_id + 1) % num_proxies;
	rewind(fp);
	fprintf(fp, "%d", proxy_id);
	fclose(fp);

	return proxy_id;
}

char *get_host(char *proxy) {
	char *colon = strstr(proxy, ":"); if (colon == NULL)
		return strdup("0.0.0.0");
	int n = colon - proxy;
	char *ret = malloc(sizeof(char) * (n + 1));
	strncpy(ret, proxy, n);
	ret[n] = '\0';
	return ret;
}

char *get_port(char *proxy) {
	char *ret = strstr(proxy, ":");

	if (!ret) {
		char message[256];
		sprintf(message, "invalid proxy format (%s)", proxy);
		perror(message);
		exit(1);
	}

	return strdup(ret + 1);
}

int is_proxy_down(const char *proc_dir, int id) {
	char proc_file[256];
	sprintf(proc_file, "%s/%s.unhealthy", proc_dir, proxies[id]);

	return access(proc_file, F_OK) == 0;
}

int main(int argc, char *argv[]) {
	if (argc <= 2) {
		fprintf(stderr, "Usage: %s <proc_dir> [<proxy-address>...]\n", argv[0]);
		return 1;
	}

	const char *proc_dir = argv[1];
	for (int i = 2; i < argc; ++i) {
		proxies[num_proxies++] = argv[i];
		if (num_proxies >= MAX_PROXIES) {
			perror("maximum allowed proxied reached");
			return 1;
		}
	}

	int proxy_id = get_new_proxy(proc_dir);
	int initial_proxy_id = proxy_id;

	while (is_proxy_down(proc_dir, proxy_id)) {
		proxy_id = get_new_proxy(proc_dir);
		if (proxy_id == initial_proxy_id) {
			fprintf(stderr, "all proxies are down.\n");
			return 1;
		}
	}

	char *proxy = proxies[proxy_id];
	char *host = get_host(proxy);
	char *port = get_port(proxy);

	char c_arg_2[256];
	snprintf(c_arg_2, sizeof(c_arg_2), "TCP:%s:%s", host, port);
	char *const command[] = {"socat", "STDIO", c_arg_2, NULL};
	execvp(command[0], command);

	free(host);
	free(port);

	return 0;
}
