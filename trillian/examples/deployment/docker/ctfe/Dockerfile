FROM golang:1.25.0-bookworm@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a as build

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

FROM gcr.io/distroless/base-debian12@sha256:d605e138bb398428779e5ab490a6bbeeabfd2551bd919578b1044718e5c30798

COPY --from=build /build/ct_server /

ENTRYPOINT ["/ct_server"]
