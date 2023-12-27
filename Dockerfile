FROM scratch
# Copy our static executable.
COPY green-button /bin/green-button
# Run the hello binary.
ENTRYPOINT ["/bin/green-button"]