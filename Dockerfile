# Use the offical Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.16.6 as builder

WORKDIR /src/keptn-locust-service

ARG version=develop
ENV VERSION="${version}"

# Force the go compiler to use modules
ENV GO111MODULE=on
ENV BUILDFLAGS=""
ENV GOPROXY=https://proxy.golang.org

# Copy `go.mod` for definitions and `go.sum` to invalidate the next layer
# in case of a change in the dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

ARG debugBuild

# set buildflags for debug build
RUN if [ ! -z "$debugBuild" ]; then export BUILDFLAGS='-gcflags "all=-N -l"'; fi

# Copy local code to the container image.
COPY . .

# Build the command inside the container.
# (You may fetch or manage dependencies here, either manually or with a tool like "godep".)
RUN GOOS=linux go build -ldflags '-linkmode=external' $BUILDFLAGS -v -o keptn-locust-service

# Use a Docker multi-stage build to create a lean production image.
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM python:3.8-slim
ENV ENV=production

ARG version=develop
ENV VERSION="${version}"

RUN pip3 install --no-cache-dir locust
RUN locust --version

# Copy the binary to the production image from the builder stage.
COPY --from=builder /src/keptn-locust-service/keptn-locust-service /keptn-locust-service

EXPOSE 8080

# required for external tools to detect this as a go binary
ENV GOTRACEBACK=all

# KEEP THE FOLLOWING LINES COMMENTED OUT!!! (they will be included within the travis-ci build)
#build-uncomment ADD MANIFEST /
#build-uncomment COPY entrypoint.sh /
#build-uncomment ENTRYPOINT ["/entrypoint.sh"]

# Run the web service on container startup.
CMD ["/keptn-locust-service"]
