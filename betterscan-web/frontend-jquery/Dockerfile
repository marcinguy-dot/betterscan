# Static jQuery frontend served by nginx. No build step, no Node, nothing to
# update on a regular cadence -- just plain HTML/CSS/JS and vendored libraries.
FROM nginx:1.28-alpine

COPY . /usr/share/nginx/html
# Vendored libs and HTML only; drop the Dockerfile/README from the web root.
RUN rm -f /usr/share/nginx/html/Dockerfile /usr/share/nginx/html/README.md

EXPOSE 80
