FROM golang:1.25.1-bookworm@sha256:916e32b41e466830cae4286075872c072d07edb5eee4dfeddfda02d0ff16a392 as build

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

FROM gcr.io/distroless/base-debian12@sha256:fa15492938650e1a5b87e34d47dc7d99a2b4e8aefd81b931b3f3eb6bb4c1d2f6

COPY --from=build /build/ct_server /

ENTRYPOINT ["/ct_server"]
