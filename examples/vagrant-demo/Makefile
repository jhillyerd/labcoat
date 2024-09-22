SHELL = /bin/sh
.PHONY = build clean destroy prepare

configs = flake.nix
srcs = demo.tape
tapes = $(srcs:.tape=.mp4)

db_ip = 192.168.33.10
web_ip = 192.168.33.11
host_ips = $(db_ip) $(web_ip)

all: clean build

clean:
	git restore $(configs)
	rm -f labcoat $(tapes)
	nixos-rebuild \
		--flake ".#db-lab" \
		--target-host root@$(db_ip) \
		--build-host localhost switch
	ssh root@$(db_ip) nix-collect-garbage -d || true

destroy:
	vagrant destroy --force
	for ip in $(host_ips); do \
		ssh-keygen -R $$ip; \
	done

prepare:
	git restore $(configs)
	vagrant up
	ssh-keyscan $(host_ips) >> $$HOME/.ssh/known_hosts
	nixos-rebuild \
		--flake ".#db-lab" \
		--target-host root@$(db_ip) \
		--build-host localhost boot
	# Kernel upgrade, reboot.
	ssh root@$(db_ip) reboot || true
	nixos-rebuild \
		--flake ".#web-lab" \
		--target-host root@$(web_ip) \
		--build-host localhost boot
	# Kernel upgrade, reboot.
	ssh root@$(web_ip) reboot || true

build: labcoat $(tapes)

labcoat:
	go build ../..

%.mp4: %.tape
	vhs $<
	git restore $(configs)
