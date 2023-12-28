FROM scratch
COPY green-button /bin/green-button
ENTRYPOINT ["/bin/green-button"]