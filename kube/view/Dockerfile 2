FROM node:20-alpine AS build-stage

WORKDIR /app
COPY package*.json ./

RUN npm install
COPY . .
RUN npm run build

FROM caddy:2-alpine

COPY --from=build-stage /app/dist /usr/share/caddy
COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
EXPOSE 443