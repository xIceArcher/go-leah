FROM golang:1.21 AS build
WORKDIR /src
COPY . .
RUN go build -o /bin/leah ./leah.go

FROM alpine
COPY --from=build /bin/leah /bin/leah
CMD ["/bin/leah"]
