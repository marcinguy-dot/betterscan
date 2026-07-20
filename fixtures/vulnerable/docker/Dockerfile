# Trivy config: running as root, outdated base, secrets-ish patterns
FROM alpine:3.14
USER root
ENV AWS_SECRET_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE_SECRET_KEY_VALUE
COPY app /app
EXPOSE 22
CMD ["/app"]
