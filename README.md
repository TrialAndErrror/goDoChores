# goDoChores

goDoChores is a web server built in Golang. You can read an overview of the project in [my writeup and initial impressions after doing the project](https://write.as/thewadegreen/experimenting-with-webservers-in-golang)

## Overview

This project is designed to help users keep track of chores that need to be performed on a regular bases. Users create chores, and then create chore reminders to tell them to do the specific chore on a predetermined interval.

## Tools Used

Overall, this project is golang for the full stack. Apart from the golang standard library, I used:
- [Chi](https://go-chi.io/#/) for routing and middleware,
- [Templ](https://templ.guide/) for template rendering, and
- [GORM](https://gorm.io/) for database models and query management
