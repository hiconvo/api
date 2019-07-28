# This is to be used in development only
FROM golang:1.12.6

RUN apt-get update
RUN apt-get install imagemagick -y

WORKDIR /var/www

COPY . .

RUN go get ./...

RUN go get github.com/oxequa/realize

VOLUME ["/var/www"]

CMD realize s
