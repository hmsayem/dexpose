FROM alpine
COPY ./dexpose /usr/local/bin/dexpose
ENTRYPOINT ["/usr/local/bin/dexpose"]