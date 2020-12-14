package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"
)

const version = "v0.1.0"
const blocksize = 4096

func main() {

	app := cli.NewApp()

	app.Name = "revtcp"
	app.Version = version
	app.Usage = "command-line utility for managing tcp reverse proxies"

	app.Commands = []*cli.Command{}
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "serve",
		Usage:     "start a reverse proxy server",
		UsageText: os.Args[0] + " serve [--from LISTEN_ADDR] --to DESTINATION",
		Aliases:   []string{"s", "srv"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen",
				Aliases: []string{"from", "l", "f"},
				Usage:   "listen address",
			},
			&cli.StringFlag{
				Name:     "to",
				Aliases:  []string{"t"},
				Required: true,
				Usage:    "origin server address",
			},
		},
		Action: func(ctx *cli.Context) error {
			listen, to := ctx.String("listen"), ctx.String("to")
			if listen == "" {
				listen = generateListenAddress()
			}
			fmt.Println("revtcp listening on: " + listen)
			logWrite(false, "Server started "+to+" <- "+listen)
			_, err := proxyServer(listen, to)
			if err != nil {
				return err
			}
			return nil
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "echo-server",
		Usage:     "start a tcp echo server",
		UsageText: os.Args[0] + " echo-server [--listen LISTEN_ADDR]",
		Aliases:   []string{"echosrv"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen",
				Aliases: []string{"port", "l", "p"},
				Usage:   "listen address",
			},
		},
		Action: func(ctx *cli.Context) error {
			listen := ctx.String("listen")
			if listen == "" {
				listen = generateListenAddress()
			}
			fmt.Println("echo-server listening on: " + listen)
			logWrite(false, "Server started "+listen)
			_, err := echoServer(listen)
			if err != nil {
				return err
			}
			return nil
		},
	})

	err := app.Run(os.Args)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

}

func generateListenAddress() string {
	rand.Seed(time.Now().UnixNano())
	return "0.0.0.0:" + strconv.Itoa(rand.Intn(55535)+1000)
}

func proxyServer(dest string, orig string) (port int, errs error) {
	srv, err := net.Listen("tcp", dest)
	if err != nil {
		errs = err
		return
	}
	defer srv.Close()
	uri, err := url.Parse(srv.Addr().String())
	if err == nil {
		port, _ = strconv.Atoi(uri.Port())
	}
	for {
		conn, err := srv.Accept()
		if err != nil {
			logWrite(true, err.Error())
		}
		go proxyServerHandler(conn, orig)
	}
}

func proxyServerHandler(destConn net.Conn, orig string) {
	logWrite(false, fmt.Sprintf(
		"New connection from %s to %s",
		destConn.RemoteAddr().String(),
		destConn.LocalAddr().String(),
	))
	defer destConn.Close()
	origConn, err := net.Dial("tcp", orig)
	if err != nil {
		logWrite(true, err.Error())
		return
	}
	origChan := make(chan []byte)
	destChan := make(chan []byte)
	go readToChan(origConn, origChan)
	go readToChan(destConn, destChan)
	var dat []byte
	for {
		select {
		case dat = <-origChan:
			destConn.Write(dat)
		case dat = <-destChan:
			origConn.Write(dat)
		}
	}
}

func echoServer(dest string) (port int, errs error) {
	srv, err := net.Listen("tcp", dest)
	if err != nil {
		errs = err
		return
	}
	defer srv.Close()
	uri, err := url.Parse(srv.Addr().String())
	if err == nil {
		port, _ = strconv.Atoi(uri.Port())
	}
	for {
		conn, err := srv.Accept()
		if err != nil {
			logWrite(true, err.Error())
		}
		go func(conn net.Conn) {
			for {
				var data []byte = make([]byte, blocksize)
				_, err := conn.Read(data)
				if err != nil {
					if err == io.EOF {
						return
					}
					logWrite(true, err.Error())
				}
				conn.Write(data)
			}
		}(conn)
	}
}

func readToChan(conn net.Conn, ch chan []byte) {
	for {
		var dat []byte = make([]byte, blocksize)
		_, err := conn.Read(dat)
		if err != nil {
			if err == io.EOF {
				return
			}
			logWrite(true, err.Error())
			return
		}
		ch <- dat
	}
}

func logWrite(isError bool, msg string) {
	if isError {
		log.Println("[ERR] " + msg)
	} else {
		log.Println("[INF] " + msg)
	}
}
