FROM nginx:alpine

COPY index.html /usr/share/nginx/html/index.html

# Custom nginx config for better caching
RUN echo 'server { \
    listen 80; \
    root /usr/share/nginx/html; \
    index index.html; \
    location / { \
        try_files $uri $uri/ /index.html; \
        add_header Cache-Control "public, max-age=3600"; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80