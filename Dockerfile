FROM golang:1.21 AS build
WORKDIR /src
COPY . .
RUN go build -o /bin/leah ./leah.go

FROM debian:stable-slim
RUN apt update
RUN apt install -y --no-install-recommends ca-certificates
RUN update-ca-certificates
COPY --from=build /bin/leah /bin/leah
CMD ["/bin/leah"]
