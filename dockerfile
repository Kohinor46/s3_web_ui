FROM --platform=linux/amd64 alpine:3.17
WORKDIR /app
COPY go.mod . 
COPY go.sum . 
COPY index.gohtml . 
COPY main.go . 
COPY s3.png . 
RUN chown 1000:1000 -R /app
RUN chmod +x -R /app/main.go
USER 1000
EXPOSE 8080
ENTRYPOINT ["/app/main.go"]