FROM artifacts.zacharyseguin.ca/docker/debian:bullseye-slim

# Install dependencies
RUN apt-get update \
    && apt-get install --no-install-recommends -y curl gnupg2 ca-certificates sqlite3 \
    && rm -rf /var/lib/apt/lists/

# Configure repository
RUN echo 'deb [arch=amd64] http://repo.powerdns.com/debian bullseye-auth-45 main' > /etc/apt/sources.list.d/powerdns.list \
    && echo 'Package: pdns-*' > /etc/apt/preferences.d/powerdns \
    && echo 'Pin: origin repo.powerdns.com' >> /etc/apt/preferences.d/powerdns \
    && echo 'Pin-Priority: 600' >> /etc/apt/preferences.d/powerdns \
    && curl https://repo.powerdns.com/FD380FBB-pub.asc | apt-key add -

# Install
RUN apt-get update \
    && apt-get install --no-install-recommends -y pdns-server pdns-backend-sqlite3 \
    && rm -rf /var/lib/apt/lists/

COPY start.sh /start.sh

EXPOSE 53
EXPOSE 8080
VOLUME [ "/data" ]
ENTRYPOINT [ "/start.sh" ]
