{ config, lib, ... }:
let
  cfg = config.roles.webserver;
in
{
  options.roles.webserver = with lib; {
    enable = mkEnableOption "Enable webserver";

    greeting = mkOption {
      type = types.str;
      description = "webserver greeting";
    };
  };

  config = lib.mkIf cfg.enable {
    services.caddy = {
      enable = true;

      virtualHosts."http://" = {
        extraConfig = ''
          respond "${cfg.greeting}"
        '';
      };
    };

    networking.firewall.allowedTCPPorts = [ 80 ];
  };
}
