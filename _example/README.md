# Example

Let us say that we have some kind of MySQL database, Redis, email worker and 
HTTP server. Intuitively, we should start these services in the following order:

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
