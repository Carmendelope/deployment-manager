
# iron/go is the alpine image with only ca-certificates added
FROM iron/go

# Now just add the binary
COPY deployment_manager /nalej/

ENTRYPOINT ["./nalej/deployment_manager"]