# Use the latest Ubuntu as the base image
FROM ubuntu:22.04

# Update the package list and install curl
RUN apt-get update && \
    apt-get install -y curl

# Install Node.js and npm using curl script from nodesource
RUN curl -sL https://deb.nodesource.com/setup_14.x | bash -
RUN apt-get install -y nodejs


CMD ["echo","hello world"]
