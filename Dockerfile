FROM golang:1.21 AS build
WORKDIR /src
COPY . .
RUN go build -o /bin/leah ./leah.go

FROM debian:stable-slim AS download-yt-dlp
RUN apt update
RUN apt install -y --no-install-recommends ca-certificates curl
RUN update-ca-certificates
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux -o /bin/yt-dlp
RUN chmod a+rx /bin/yt-dlp

FROM debian:stable-slim
RUN apt update
RUN apt install -y --no-install-recommends ca-certificates
RUN update-ca-certificates
RUN apt install -y ffmpeg
COPY --from=build /bin/leah /bin/leah
COPY --from=download-yt-dlp /bin/yt-dlp /bin/yt-dlp  
CMD ["/bin/leah"]
