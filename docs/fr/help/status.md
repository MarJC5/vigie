# Lire l'état d'un site, et ignorer un constat

Chaque relevé donne un état au site, affiché dans la liste des sites et sur sa page. C'est le pire
de ce que le relevé a trouvé.

## Les états

- **Urgent** : à traiter tout de suite. Une vulnérabilité connue grave ou exploitée, des fichiers
  de WordPress modifiés, un fichier PHP dans le dossier des téléversements, un certificat ou un
  domaine sur le point d'expirer.
- **À surveiller** : une mise à jour en attente, un administrateur sans double authentification,
  PHP qui n'est plus maintenu, et d'autres points à régler quand vous pouvez.
- **Pas entièrement vérifié** : une vérification n'a pas pu se faire. L'onglet Sécurité dit
  laquelle, et pourquoi.
- **À jour** : rien à signaler.

## Lire les constats

L'onglet **Sécurité** d'un site liste ce que le relevé a trouvé, par niveau, avec les faits et les
liens vers les sources. **Vérifier les fichiers** compare à nouveau les fichiers avec
wordpress.org, ce que Vigie fait sinon une fois par jour.

## Ignorer un constat

Un constat que vous connaissez peut être ignoré : **Ignorer…**, puis soit **jusqu'à la prochaine
version** de son extension, de son thème, de WordPress ou de PHP, soit **définitivement**, avec une
raison. Il reste listé, grisé, et ne compte plus dans l'état. **Ne plus ignorer** le rétablit.

Pour aller plus loin : [comment les constats sont classés](../security.md).
