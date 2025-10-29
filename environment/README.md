# Readme for the execution evironment

The external plug-ins can be installed and used in the execution environment without having to install them locally on your own computer.
The desired programs must be listed in the Dockerfile during the ```apt``` installation or installed interactively in the container.

## Setup

Install docker with docker compose, move to the environment folder and execute ```docker compose up```.
Afterwards use the docker commands to access the container or use the Docker plug-in for Visual Studio Code from Microsoft to attach VSC and/or the shell to the container.

## Todo:
- [ ] Install plug-in automatically via plug-in YAML.
- [ ] Setup ssh, scp and ufw inside the docker.

