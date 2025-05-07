FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git gcc musl-dev zlib-dev zlib-static
#RUN apt-get -y install python3-pip make git zip gcc g++ zlib1g zlib1g-dev

COPY . /build

WORKDIR /build

RUN go mod download

RUN CGO_ENABLED=1 go build -tags "osusergo netgo static_build lambda.norpc" -ldflags '-extldflags "-static -lz"' -o bootstrap

FROM alpine:latest AS runner

RUN apk add --no-cache ca-certificates

COPY --from=builder /build/bootstrap /usr/local/bin/bootstrap

ENTRYPOINT ["bootstrap"]