FROM scratch
COPY geo-server /
COPY index.html /cmd/geo-server/index.html

ENV PORT=8080
EXPOSE $PORT
ENTRYPOINT ["/geo-server"]
CMD [""]
