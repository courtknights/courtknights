# Local OAuth2 fake for Google and GitHub (ADR-014). Development use only.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /oauthfake .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /oauthfake /oauthfake
EXPOSE 8080
ENTRYPOINT ["/oauthfake"]
