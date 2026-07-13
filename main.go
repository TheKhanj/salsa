package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type Listener interface {
	Listen() error
	Shutdown()
}

var (
	VERSION  = "dev"
	PROC_DIR string
)

const (
	DEFAULT_LISTEN string = "0.0.0.0:5743"
)

func showHelpMessage() int {
	fmt.Println("NAME")
	fmt.Println("  salsa - TCP/UDP load balancer")
	fmt.Println()
	fmt.Println("SYNOPSIS")
	fmt.Println("  salsa [-u] [-l <listening-address>] <backends...>")
	fmt.Println()
	fmt.Println("OPTIONS")
	flag.PrintDefaults()
	fmt.Println("EXAMPLES")
	fmt.Println("  salsa -l :4000 127.0.0.1:3000 127.0.0.1:3001")
	fmt.Println("  salsa -u -l :5000 127.0.0.1:3000 127.0.0.1:3001")

	return 0
}

func showManPage() int {
	cmd := exec.Command("man", "salsa")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("error opening man page: %s\n", err.Error())
		return 1
	}
	return 0
}

func getBackends(backends []string) []Backend {
	var ret []Backend

	for _, backend := range backends {
		path := fmt.Sprintf(
			PROC_DIR+"/backends/%s", backend,
		)
		ret = append(ret, NewBackend(backend, path))
	}

	return ret
}

func initProcDir(backends []Backend) error {
	err := os.MkdirAll(PROC_DIR, 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(PROC_DIR+"/backends", 0755)
	if err != nil {
		return err
	}

	for i := range backends {
		backend := &backends[i]
		os.Mkdir(backend.dir, 0755)
		err = os.WriteFile(
			backend.dir+"/score",
			([]byte)(fmt.Sprintf("%d\n", backend.GetScore())),
			0644,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanup(l Listener) {
	l.Shutdown()
	os.RemoveAll(PROC_DIR)
	os.Exit(0)
}

func run() int {
	if uid := syscall.Getuid(); uid == 0 {
		PROC_DIR = fmt.Sprintf("/var/run/salsa/%d", os.Getpid())
	} else {
		PROC_DIR = fmt.Sprintf("/run/user/%d/salsa/%d", uid, os.Getpid())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	listen := flag.String("l", DEFAULT_LISTEN, "listening address")
	udpMode := flag.Bool("u", false, "use UDP mode")
	help := flag.Bool("h", false, "show help message")
	manPage := flag.Bool("help", false, "show man page")
	version := flag.Bool("v", false, "show version")

	flag.Parse()
	backends := flag.Args()

	if *manPage == true {
		return showManPage()
	}

	if *help == true {
		return showHelpMessage()
	}

	if *version == true {
		fmt.Println(VERSION)

		return 0
	}

	backendsList := getBackends(backends)

	var l Listener
	if *udpMode {
		ul := NewUDPListener(*listen, backendsList)
		l = &ul
	} else {
		tl := NewTCPListener(*listen, backendsList)
		l = &tl
	}

	defer cleanup(l)

	err := initProcDir(backendsList)
	if err != nil {
		log.Println(err)
		return 2
	}

	serverStopped := make(chan struct{})
	go func() {
		err := l.Listen()
		if err != nil {
			log.Println(err.Error())
		}
		close(serverStopped)
	}()

	select {
	case <-sigs:
		return 0
	case <-serverStopped:
		return 3
	}
}

func main() {
	os.Exit(run())
}
