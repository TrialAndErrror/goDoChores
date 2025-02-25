# goDoChores

goDoChores is a web server built in Golang. You can read an overview of the project in [my writeup and initial impressions after doing the project](https://write.as/thewadegreen/experimenting-with-webservers-in-golang)

## Overview

This project is designed to help users keep track of chores that need to be performed on a regular bases. Users create chores, and then create chore reminders to tell them to do the specific chore on a predetermined interval.

## Tools Used

Overall, this project is golang for the full stack. Apart from the golang standard library, I used:
- [Chi](https://go-chi.io/#/) for routing and middleware,
- [Templ](https://templ.guide/) for template rendering, and
- [GORM](https://gorm.io/) for database models and query management

## Deployment Instructions

Run in development:
- Clone the repo
- `templ generate` to compile the templates
- `go run . create-user` to create a user for yourself
- `go run .` to just run the dev server (it will default to port 3000)

Basic deployment:
- `templ generate` to compile the templates
- `go build .` to build a binary
- Copy the binary wherever you want it
- Copy the .env.example file to .env and fill out a specific port you want to use
- `./goDoChores create-user` to create the user (or whatever you called the binary, if you specified a name)
- `./goDoChores` to run the server

See `./dev-scripts` for shell scripts that I use for building binaries and deploying to my server
