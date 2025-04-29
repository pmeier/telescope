FROM golang:1.23-bookworm AS build

WORKDIR /src

COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/telescope .

FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/telescope /

ENV TELESCOPE_HOST=0.0.0.0

ENTRYPOINT ["/telescope"]
CMD ["observe"]
HEALTHCHECK CMD ["/telescope", "health"]
