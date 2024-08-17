package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var PROC_DIR string

const (
	DEFAULT_INTERVAL int64  = 5
	DEFAULT_LISTEN   string = "0.0.0.0:5743"
)

func showHelp() {
	flag.PrintDefaults()
}

func showManPage() {
	// TODO: implement
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

func initProcDir(l *TCPListener) error {
	err := os.MkdirAll(PROC_DIR, 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(PROC_DIR+"/backends", 0755)
	if err != nil {
		return err
	}

	for _, backend := range l.Backends {
		err = os.WriteFile(
			backend.filePath,
			([]byte)(fmt.Sprintf("%d", backend.Weight)),
			0755,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanup(l *TCPListener) {
	l.Shutdown()
	os.RemoveAll(PROC_DIR)
	os.Exit(0)
}

func main() {
	if uid := syscall.Getuid(); uid == 0 {
		PROC_DIR = fmt.Sprintf("/var/run/salsa/%d", os.Getpid())
	} else {
		PROC_DIR = fmt.Sprintf("/run/user/%d/salsa/%d", uid, os.Getpid())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	listen := flag.String("l", DEFAULT_LISTEN, "listening address")
	interval := flag.Int64("t", DEFAULT_INTERVAL, "backend check interval in seconds")
	help := flag.Bool("h", false, "show help message")
	manPage := flag.Bool("help", false, "show man page")

	flag.Parse()
	backends := flag.Args()

	if *manPage == true {
		showManPage()
		return
	}

	if *help == true {
		showHelp()
		return
	}

	listener := NewTCPListener(
		*listen,
		getBackends(backends),
		(time.Duration)(*interval)*time.Second,
	)

	defer cleanup(&listener)

	err := initProcDir(&listener)
	if err != nil {
		log.Println(err)
		return
	}

	go func() {
		listener.Listen()
		if err != nil {
			log.Println(err.Error())
		}
	}()

	<-sigs
}
