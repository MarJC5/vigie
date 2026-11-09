# Publier Vigie

Une version est publiée par le propriétaire, sur un Mac : l'app macOS y est construite, et la clé
qui signe chaque version n'en sort jamais. GitHub Actions construit les paquets Linux à partir de
l'étiquette, sans aucun secret. Un Vigie installé ne fait confiance à une nouvelle version que si
elle est signée avec la clé qu'il contient.

## Ce que contient une version

| Fichier | Pour |
| --- | --- |
| `Vigie-x.y.z.dmg` | Installer sur macOS, et le cask Homebrew |
| `Vigie-x.y.z-macos.zip` | Vigie qui se met à jour sur macOS (l'app universelle) |
| `Vigie-x.y.z-x86_64.AppImage`, `-aarch64.AppImage` | Linux, et Vigie qui s'y met à jour |
| `Vigie-x.y.z-amd64.deb`, `-arm64.deb`, `.rpm`, `.pkg.tar.zst` | Les gestionnaires de paquets Linux |
| `SHA256SUMS` | La somme de contrôle de chaque fichier |
| `update.json` | Ce que Vigie lit pour apprendre une nouvelle version : chaque fichier qu'il installe, signé |

Vigie ne lit que le `update.json` de la dernière version publiée : jamais un brouillon ni une
préversion.

## Une fois : la paire de clés

```sh
go run ./cmd/vigie-release keygen ~/quelque-part-prive
```

Elle écrit la clé privée (`vigie-release.key`, lisible par vous seul) et la clé publique, qu'elle
affiche. Gardez la clé privée hors du dépôt, avec une copie dans votre gestionnaire de mots de
passe : une version signée avec une autre clé est refusée par chaque Vigie installé. La clé
publique va dans `internal/buildinfo/update.go` (`UpdateKey`). Une clé n'est jamais écrasée.

## Publier une version

Le travail se fait sur `develop`. `main` ne bouge qu'à la publication d'une version : elle montre
toujours la dernière version publiée, et son README la version que les gens installent.
Une règle du dépôt n'accepte un commit sur `main` qu'une fois les deux jobs de CI réussis sur lui :
un commit fait avec `[skip ci]` ne peut pas atteindre `main`.

1. Sur `develop`, dans `CHANGELOG.md`, déplacer ce qui est sous `Unreleased` dans une nouvelle
   section `## [x.y.z] - AAAA-MM-JJ` : son texte devient les notes de la version, dans la fenêtre
   aussi. Commiter et pousser `develop`, et attendre sa CI.
2. Amener `main` à `develop`, puis poser l'étiquette et pousser :

   ```sh
   git switch main
   git merge --ff-only develop
   git tag vx.y.z
   git push origin main vx.y.z
   git switch develop
   ```

   L'étiquette lance le workflow Release : les vérifications de la CI, puis les paquets Linux pour
   x86_64 et aarch64.
3. Sur le Mac, depuis l'étiquette :

   ```sh
   make release VERSION=x.y.z KEY=~/quelque-part-prive/vigie-release.key
   ```

   Il attend le workflow, construit l'app universelle, son zip et le DMG, récupère les paquets
   Linux, signe chaque fichier, vérifie les signatures avec Wails et la clé de l'app, et crée une
   version **brouillon** avec tous les fichiers.
4. Relire le brouillon sur GitHub, puis le publier : `gh release edit vx.y.z --draft=false`. Dès
   lors, les Vigie installés la proposent.
5. Mettre à jour le cask du tap `MarJC5/homebrew-tap` (`Casks/vigie.rb`) avec la version et le
   SHA-256 qu'affiche `make release`, ou copier `dist/vx.y.z/vigie.rb`.

## Essayer une mise à jour avant de publier

Une mise à jour s'essaie depuis un serveur sur votre Mac, signée avec la même clé : construire et
installer une version inférieure, puis en servir une supérieure.

```sh
make app VERSION=0.0.1          # copier bin/Vigie.app dans /Applications
make app VERSION=0.0.2
mkdir -p dist/try && ditto -c -k --norsrc --noextattr --noacl --keepParent bin/Vigie.app dist/try/Vigie-0.0.2-macos.zip
go run ./cmd/vigie-release sign -key ~/quelque-part-prive/vigie-release.key -version 0.0.2 \
  -base http://127.0.0.1:8000 dist/try/Vigie-0.0.2-macos.zip
python3 -m http.server 8000 --bind 127.0.0.1 --directory dist/try
VIGIE_UPDATE_MANIFEST=http://127.0.0.1:8000/update.json /Applications/Vigie.app/Contents/MacOS/Vigie </dev/null
```

`VIGIE_UPDATE_MANIFEST` ne change que l'endroit où le manifeste est lu : la signature est vérifiée
avec la clé de l'app quand même. L'entrée lue depuis `/dev/null` garde l'app dans ce processus,
avec la variable, au lieu de passer la main au Finder. Dans Réglages, accepter les recherches
extérieures si elles sont demandées, puis À propos, « Rechercher une mise à jour ».

## Le WP-CLI qu'installe Vigie

Vigie installe une version de WP-CLI sur les comptes qui n'en ont pas (ADR-0039), fixée dans
`internal/wpcliinstall` avec sa taille et son SHA-512. Pour en fixer une plus récente, vérifier
d'abord sa signature avec la clé de publication de WP-CLI, dans un trousseau à part :

```sh
v=2.13.0
mkdir -p /tmp/wpcli && cd /tmp/wpcli
curl -LO https://github.com/wp-cli/wp-cli/releases/download/v$v/wp-cli-$v.phar \
     -LO https://github.com/wp-cli/wp-cli/releases/download/v$v/wp-cli-$v.phar.asc \
     -LO https://github.com/wp-cli/wp-cli/releases/download/v$v/wp-cli-$v.phar.sha512
mkdir -m 700 gnupg
curl -L https://raw.githubusercontent.com/wp-cli/builds/gh-pages/wp-cli.pgp | GNUPGHOME=gnupg gpg --import
GNUPGHOME=gnupg gpg --verify wp-cli-$v.phar.asc wp-cli-$v.phar
```

La signature doit être bonne, de la clé `63AF 7AA1 5067 C056 16FD DD88 A3A2 E8F2 26F0 BC06`
(« Verifying WP-CLI Downloads » de WP-CLI). Ensuite, mettre `Version`, `SHA512` et `Size` dans
`internal/wpcliinstall/wpcliinstall.go`, le fichier `.sha512` dans son `testdata` (le test le lit),
et la même version et la même empreinte dans le Dockerfile du banc
(`internal/integration/testdata/wordpress/Dockerfile`), pour que le test d'intégration l'installe.

## La version d'un build

`make app VERSION=x.y.z` inscrit la version dans le binaire (`vigie version`), dans l'`Info.plist`
de l'app et dans les paquets Linux. Sans elle, le build indique `dev` et ne cherche jamais de mise
à jour.
