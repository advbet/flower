# Flower
`flower` (_flow er_) is a package that provides an idiomatic, simple and an expressive way of defining how services (goroutines) should be started and gracefully closed. 

[![Go Reference](https://pkg.go.dev/badge/github.com/advbet/flower.svg)](https://pkg.go.dev/github.com/advbet/flower)

## Features

* Simple API
* Uses `context.Context`
* Hookable functions (e.g. `BeforeServiceStart`)
* Configurable panic recovery

## Installing

`go get -u github.com/advbet/flower`

## Idea

Let us say that we have some kind of MySQL database, Redis, email worker and 
a HTTP server. Intuitively, we should start these services in the following 
order:

1) MySQL (it is most likely that all servers need it for some reason or another)
2) Redis (cache can be in between our application and persistence layer)
3) Email worker (now that we have our persistence services started, we can 
start email worker)
4) HTTP server (since all services are started, we can now serve HTTP requests)

Naturally, the order in which the services are closed should be similar to 
the order in which the services were started in. However, they should start 
in opposite direction. This means that our services run in a LIFO manner; 
services should close in the opposite direction:

1) HTTP server (we finish current requests and close all incoming ones)
2) Email worker (finish sending last emails)
3) Redis (close cache connection)
4) MySQL (we no longer need the database)

Using `flower`, this flow could be expressed as following:

```go
package main

import (
	_ "github.com/go-sql-driver/mysql"	
	"github.com/go-redis/redis/v8"
	"os/signal"
	"os/syscall"
	"sync"
	"net/http"
	"time"
	"fmt"
	"github.com/advbet/flower"
)

type emailWorker struct {
	db *sql.DB
	redis *redis.Client
}

func (ew *emailWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
			case <- ctx.Done():
				// service closed, return
			case <- ticker.C:
				// send email
		}
	}
}

type server struct {
	port int
}

func (s *server) Run(ctx context.Context) {
	srv := &htp.Server {
		Addr: fmt.Sprintf(":%d", s.port),
	}

	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		srv.ListenAndServe()
	}()

	select {
		case <- ctx.Done():
			srv.Shutdown(context.Background()) //TODO: timeout context
			return
		case <- doneCh: // unexpected
	}
}

func main() {
	db, err := sql.Open("mysql", "dsn")	
	if err != nil {
		panic(err)
	}

	defer db.Close()

	redis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})	

	defer redis.Close()

	ew := &emailWorker {
		db: db,
		redis: redis,
	}

	closeCh := make(chan os.Signal, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer cancel()

	opts := flower.Options {
		OnServiceStarting: func(s string) {
			fmt.Println(s)
		},
	}

	s := &server {
		port: 4212,
	}

	flower.Run(ctx, opts,
		flower.ServiceGroup{
			"emailWorker": ew,
		},
		flower.ServiceGroup{
			"server": s,
		},
	)

	fmt.Println("gracefully closed")
}
```

The key concept comes from flower's `ServiceGroup` structure. It is a unit that is composed 
out of many services (goroutines). Within a service group, all goroutines start/close 
nondeterministically. However, the order of service groups are respected; each service 
group will start only after the previous group has started, and will close only when 
the superseding service group is closed.

## Important

Project is still in experimental phase, breaking changes may happen.
