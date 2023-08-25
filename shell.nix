{ pkgs ? import <nixpkgs> {} }:

let
	gotk4-nix = pkgs.fetchFromGitHub {
		owner  = "diamondburned";
		repo   = "gotk4-nix";
		rev    = "b186ac5419c22f4b75b1bdc35ef4fd9349c6b65e";
		sha256 = "1pfx0p4w56d7pa1l9ipqfq52znfl7slc2wbjfqddq1jv1fp9z43q";
	};

in import "${gotk4-nix}/shell.nix" {
	base = {
		pname = "chatkit";
		version = "dev";
	};
	usePatchedGo = false;
}
