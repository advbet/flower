# Flower
`flower` (_flow er_) is a package that provides an idiomatic, simple and an expressive way of defining how services (goroutines) should be started and gracefully closed. 

[![Go Reference](https://pkg.go.dev/badge/github.com/advbet/flower.svg)](https://pkg.go.dev/github.com/advbet/flower)
[![Build Status](https://github.com/advbet/flower/actions/workflows/go.yml/badge.svg)](https://github.com/advbet/flower/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/advbet/flower/badge.svg?branch=main)](https://coveralls.io/github/advbet/flower?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/advbet/flower)](https://goreportcard.com/report/github.com/advbet/flower)

## Features

* Simple API
* Uses `context.Context`
* Hookable functions (e.g. `BeforeServiceStart`)
* Configurable panic recovery

## Installing

`go get -u github.com/advbet/flower`

## Concept

The key concept comes from flower's `ServiceGroup` structure. It is a unit that is composed 
out of many services (goroutines). Within a service group, all goroutines start/close 
nondeterministically. However, the order of service groups are respected; each service 
group will start only after the previous group has started, and will close only when 
the superseding service group is closed. See [_example](https://github.com/advbet/flower/blob/main/_example/) for more details.

## Important

Project is still in experimental phase, breaking changes may happen.
