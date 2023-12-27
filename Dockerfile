# FROM scratch
FROM alpine
COPY green-button /bin/green-button
ENTRYPOINT ["/bin/green-button"]