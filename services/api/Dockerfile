FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# nodynamic: gen2brain/webp would otherwise dlopen a system libwebp via purego, which links the
# binary dynamically and breaks it in the libc-less distroless image.
RUN CGO_ENABLED=0 go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
# Guard: fail the build if any binary is not static.
RUN apk add --no-cache file \
 && for b in /out/api /out/worker; do file "$b" | grep -q 'statically linked' || { file "$b"; exit 1; }; done

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/worker /usr/local/bin/
EXPOSE 8080
# Railway overrides the command per service (api / worker).
CMD ["api"]
