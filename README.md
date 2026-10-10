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
- Copy `.env.example` to `.env` and set `JWT_SECRET` to a long random string
- `templ generate` to compile the templates
- `go run . create-user` to create a user for yourself (this also creates the database on a fresh install)
- `go run .` to start the dev server (it will default to port 3000)

Basic deployment:
- `templ generate` to compile the templates
- `go build .` to build a binary
- Copy the binary wherever you want it
- Copy the .env.example file to .env and fill in `JWT_SECRET` (required), `PORT`, and optionally `DB_PATH` (SQLite file, default `test.db`) and `COOKIE_SECURE=true` if you serve over HTTPS
- `./goDoChores create-user` to create the user (or whatever you called the binary, if you specified a name)
- `./goDoChores` to run the server

## Managing users

There is no sign-up page, so users are managed from the command line. Every command creates the database and its tables first if they don't exist yet.

- `./goDoChores create-user` asks for a username, email and password (the password prompt is hidden and asked twice). Pass `-u/--username`, `-e/--email` and `-p/--password` to skip the questions; note that `--password` can end up in your shell history. Running it for an existing username updates that user's email and password.
- `./goDoChores list-users` shows every user with their ID, email, how many chores, reminders and tasks they have, and when they were created.
- `./goDoChores delete-user <username>` permanently deletes a user **and all of their chores, reminders and tasks**. You have to type the username to confirm, or pass `-y/--yes` to skip that. Their logins stop working immediately, and the username and email can be used again.

See `./.dev_scripts` for shell scripts that I use for building binaries and deploying to my server
