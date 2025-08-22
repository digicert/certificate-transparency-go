FROM golang:1.24.4-bookworm@sha256:8faa537d0690017d8000e38e1ed9ed4c5990d40a3022ce2596a825c6c87f62c4 as build

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS

# Add GitHub token and GOPRIVATE for private module access
ARG GITHUB_TOKEN
ENV GITHUB_TOKEN=${GITHUB_TOKEN}
ENV GOPRIVATE=github.com/digicert/ctutils
ENV GONOSUMDB=github.com/digicert/ctutils
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build ./trillian/ctfe/ct_server

FROM gcr.io/distroless/base-debian12@sha256:201ef9125ff3f55fda8e0697eff0b3ce9078366503ef066653635a3ac3ed9c26

COPY --from=build /build/ct_server /

ENTRYPOINT ["/ct_server"]
