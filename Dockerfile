# Use a base image with Go installed
FROM golang:1.23.4

# Set the working directory inside the container
WORKDIR /app

# Copy the Go source code and assets into the container
COPY . .

# Build the Go application
RUN go build -o media-server .

EXPOSE 8000

# Command to run the application
CMD ["./media-server"]
