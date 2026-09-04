FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /BOT_MAI ./

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /BOT_MAI /BOT_MAI

EXPOSE 8080

ENTRYPOINT ["/BOT_MAI"]
