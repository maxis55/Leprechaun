# syntax=docker/dockerfile:1

FROM golang:alpine

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY . /app

RUN go install

# Run
CMD [ "/go/bin/leprechaun" ]