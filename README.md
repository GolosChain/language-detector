# Overview

A JSON/HTTP service.

Takes input text and tries to determine the language. Uses cld2: https://github.com/CLD2Owners/cld2.git

# How to Build

Checkout Dockerfile

# How to Run

    $ ./language-detector

Here is an example request:

    $ curl -d '{"request": [{"text": "This is an example input message."}]}' -H 'content-type: application/json' localhost:3000

# How to Test

    $ make test

# Notes

- Generate known languages file in data/ using gen_codes.py

