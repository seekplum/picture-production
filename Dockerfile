FROM golang:1.16-alpine as build
WORKDIR /app
ENV GOPROXY https://mirrors.aliyun.com/goproxy/
ADD go.mod /app
ADD go.sum /app
RUN go mod download
ADD main.go /app
ADD images /app/images
ENV TMPDIR /tmp-images
RUN mkdir -p ${TMPDIR}
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM scratch
ENV TMPDIR /tmp-images
COPY --from=build /app/app /app
COPY --from=build /app/images /images
COPY --from=build ${TMPDIR} ${TMPDIR}
EXPOSE 8080
CMD [ "/app" ]
