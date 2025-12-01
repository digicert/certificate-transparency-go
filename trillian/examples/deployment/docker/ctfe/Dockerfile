FROM golang:1.25.4-bookworm@sha256:e17419604b6d1f9bc245694425f0ec9b1b53685c80850900a376fb10cb0f70cb as build

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS

# Add GitHub token and GOPRIVATE for private module access
# ARG GITHUB_TOKEN - Removed in favor of Docker secrets
ENV GOPRIVATE=github.com/digicert/ctutils
ENV GONOSUMDB=github.com/digicert/ctutils

WORKDIR /build

COPY go.mod .
COPY go.sum .
# Configure git with token, download modules, then clean up credentials in the same layer
# We use a configurable username to support both GitHub App tokens (x-access-token) and PATs (custom or default)
ARG GITHUB_USER=x-access-token
# Use secrets mount to avoid leaking token in image history
RUN --mount=type=secret,id=github_token \
    git config --global url."https://${GITHUB_USER}:$(cat /run/secrets/github_token)@github.com/".insteadOf "https://github.com/" && \
    go mod download && \
    rm -f ~/.gitconfig
COPY . .

RUN go build ./trillian/ctfe/ct_server

FROM gcr.io/distroless/base-debian12@sha256:9e9b50d2048db3741f86a48d939b4e4cc775f5889b3496439343301ff54cdba8

COPY --from=build /build/ct_server /

ENTRYPOINT ["/ct_server"]
