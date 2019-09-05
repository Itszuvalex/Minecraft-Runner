FROM openjdk:8-jre

EXPOSE 25565/tcp
EXPOSE 25565/udp
EXPOSE 8080/tcp

COPY mcrunner /srv/mcrunner/

WORKDIR /srv/mcrunner

ENTRYPOINT /srv/mcrunner/mcrunner
