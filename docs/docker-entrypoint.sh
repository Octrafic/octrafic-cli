#!/bin/sh
# Substitute only PORT variable in nginx config
sed "s/\${PORT}/${PORT}/g" /etc/nginx/conf.d/default.conf > /tmp/default.conf
mv /tmp/default.conf /etc/nginx/conf.d/default.conf
exec nginx -g 'daemon off;'
