# Start from the latest golang base image
FROM golang:latest

# Add Maintainer Info
LABEL maintainer="Zachary Kent <ztkent@gmail.com>"

# Pull the repository
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main .

# Expose app port to the outside world
ARG APP_PORT=8080
ENV APP_PORT=$APP_PORT
EXPOSE 8081

# Command to run the executable
CMD ["./main"]