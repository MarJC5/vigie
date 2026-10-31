# Ce que Vigie lit sur un serveur, et ce qu'elle envoie

Vigie se connecte à vos hébergements par SSH, avec les clés de votre agent SSH, et ne demande
jamais de mot de passe. Un relevé ne fait que lire : rien n'est écrit sur le serveur.

## Ce que lit un relevé

Les versions de WordPress, de PHP, des extensions et des thèmes, les mises à jour en attente, les
fichiers comparés avec wordpress.org, les administrateurs et leur double authentification, quelques
réglages de `wp-config.php`. **Jamais** un mot de passe, une clé de `wp-config.php` ni le contenu de
votre base de données.

## Ce qui est demandé depuis votre ordinateur

Certaines vérifications passent par un service extérieur : les vulnérabilités connues, les
extensions fermées sur wordpress.org, l'expiration du domaine et du certificat. Vigie vous demande
d'abord votre accord, avant le premier relevé qui en a besoin.

- Ces services reçoivent les noms de vos extensions et thèmes, les versions de WordPress et de PHP,
  et vos noms de domaine. **Jamais** les versions de vos extensions, vos accès SSH ni rien de ce
  qui est lu dans vos bases de données.
- Chacun peut être coupé dans **Réglages, Recherches extérieures**. Ses vérifications sont alors
  « Non vérifié ».

## Ce qu'écrit une mise à jour

Seulement la mise à jour elle-même, et ses sauvegardes dans `~/.vigie/backups/` (voir « Mettre à
jour un site, et revenir en arrière »).

Pour aller plus loin : [ce que Vigie lit, demande et envoie](../security.md).
