# This is to be used in development only
FROM golang:1.13.4

RUN apt-get update
RUN apt-get install imagemagick -y

WORKDIR /var/www

COPY . .

RUN go get ./...

RUN go get -u github.com/cosmtrek/air

VOLUME ["/var/www"]

CMD ["air"]
