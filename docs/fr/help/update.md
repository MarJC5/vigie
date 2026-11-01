# Mettre à jour un site, et revenir en arrière

Vigie met à jour WordPress, les extensions, les thèmes et les traductions d'un site, ou de tous
les sites d'un hébergement ou d'un dossier.

## Mettre à jour

1. Ouvrez le site, onglet **Mises à jour**, et cochez ce qu'il faut mettre à jour.
2. Lisez l'**aperçu** : ce qui change, et ce qui est sauvegardé avant.
3. Lancez la mise à jour. Vigie sauvegarde la base de données et les dossiers concernés, met à
   jour un élément à la fois, et **vérifie le site** après chacun.

Au premier problème, la mise à jour s'arrête : les éléments suivants ne sont pas lancés.

Pour tous les sites d'un hébergement ou d'un dossier, ouvrez-le et choisissez **Tout mettre à
jour** : un aperçu par site, puis chaque site est mis à jour comme ci-dessus.

Une version gardée exprès : **Ignorer…** sur sa ligne, jusqu'à la version suivante ou
définitivement avec une raison. Elle ne compte plus, et **Tout mettre à jour** la laisse de côté.

## Revenir en arrière

Quand la dernière mise à jour d'un site a échoué, **Revenir en arrière** remet l'élément en échec
tel qu'il était. Pour WordPress lui-même, la base de données revient aussi : ce qui y a été écrit
entre-temps (commandes, commentaires) est perdu.

## Les sauvegardes

Vigie garde sur le serveur celles des 3 dernières mises à jour de chaque site, dans
`~/.vigie/backups/`. Elles contiennent toute la base de données : si ce dossier est accessible
depuis le web, supprimez-les.

Pour aller plus loin : [ce que Vigie écrit sur un serveur](../updates.md).
