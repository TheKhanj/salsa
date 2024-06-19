# salsa

**TCP Load Balancer Proxy with Health Checks**

## Overview

`salsa` is a versatile tool for creating a TCP load balancer proxy with integrated health checks for underlying proxies. It listens on a specified address and distributes TCP requests to healthy proxies using a round-robin algorithm.

## Features

- **Load Balancing**: Distributes incoming TCP requests across multiple proxies.
- **Health Checks**: Ensures only healthy proxies receive requests based on configurable health checks.
- **Customization**: Supports customizable health check intervals and commands.

## Installation

1. **Download**:
   Clone the repository from GitHub:
   ```
   git clone https://github.com/thekhanj/salsa.git
   cd salsa
   ```

2. **Build and Install**:
   Compile and install the `salsa` executable:
   ```
   make
   sudo make install
   ```

## Usage

### Command-Line Options

```
salsa [OPTIONS...] -l <listening-address> [<proxy-address>...]
```

- **`-l <listening-address>`**: Specifies the address on which the proxy listens.

#### Options

- **`-t <time>`**: Interval between health checks (default: 5 seconds).
- **`-c <check-proxy>`**: Custom command to check proxy health.
- **`-h, --help`**: Display help message and exit.

#### Examples

- Start `salsa` with default settings:
  ```
  salsa -l 127.0.0.1:8080 192.168.1.1:9090 192.168.1.2:9090
  ```

- Configure `salsa` with a custom health check interval:
  ```
  salsa -t 10 -l 127.0.0.1:8080 192.168.1.1:9090 192.168.1.2:9090
  ```

- Use a custom health check command:
  ```
  salsa -c 'test 1 -eq 1' -l 127.0.0.1:8080 192.168.1.1:9090 192.168.1.2:9090
  ```

## Reporting Issues

For bug reports or feature requests, please visit our [GitHub Issues Page](https://github.com/thekhanj/salsa/issues).
