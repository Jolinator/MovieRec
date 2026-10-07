FROM node:22-alpine AS build

WORKDIR /app

<<<<<<< HEAD
COPY frontend/package.json ./
RUN npm install

COPY frontend/ft.env ./.env
COPY frontend/ ./
RUN npm run build

FROM nginx:alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
=======
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY frontend/ft.env ./.env
COPY frontend/src ./src
COPY frontend/index.html ./
#COPY frontend/vite.config.* ./ Maybe necessary for vite config, but not needed for now
RUN npm run build

FROM nginx:alpine

COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
>>>>>>> c27a1b0 (new movie database)
