FROM node:22-alpine AS deps

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build

WORKDIR /src
COPY --from=deps /src/node_modules ./node_modules
COPY . .
RUN npm run build

FROM nginx:1.25-alpine

COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
