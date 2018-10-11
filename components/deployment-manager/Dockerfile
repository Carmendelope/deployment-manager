
# iron/go is the alpine image with only ca-certificates added
FROM iron/go

# Now just add the binary
COPY deployment-manager /nalej/

ENTRYPOINT ["./nalej/deployment-manager"]