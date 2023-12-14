# Stage 1: Build
FROM node:16-alpine AS build

WORKDIR /app
COPY package*.json ./
RUN yarn install --frozen-lockfile
COPY . .
RUN yarn build

# Stage 2: Production Image
FROM node:16-alpine

RUN apk update && apk add --upgrade libcrypto3 libssl3

WORKDIR /app
COPY --from=build /app/dist ./dist
COPY package*.json ./
RUN yarn install --production --frozen-lockfile

# Use a non-root user
USER node

# Add the path to ts-node-dev (adjust the path as needed)
ENV PATH="/app/node_modules/.bin:${PATH}"

EXPOSE 3000

CMD ["node", "/app/dist/index.js"]
