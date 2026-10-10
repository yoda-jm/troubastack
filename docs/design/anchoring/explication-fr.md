# Les annotations qui « suivent » le texte : ce qui se passe, et ce que je propose

*À lire en 5 minutes. Exemple : la grille de démonstration « The Open Road ».*

## 1. De quoi on parle

Il y a deux sortes de partitions :

- **Les PDF que tu importes.** Rien ne bouge jamais. Une annotation reste exactement où tu l'as dessinée. Ce document ne les concerne pas.
- **Les grilles écrites en texte** (paroles + accords), que le serveur met en page. Si tu ajoutes un couplet, tout ce qui est en dessous **descend**. Une annotation qui ne bougerait pas se retrouverait alors sur les mauvais mots.

Une annotation **liée** est une annotation que le serveur rattache à une ligne du texte, pour la déplacer avec elle quand la mise en page change.

## 2. Ce qui se passe aujourd'hui (et pourquoi tu perds confiance)

On a testé chaque forme et chaque situation : **225 cas**. Aujourd'hui, **140 se passent mal**. Voici les principaux, sur l'exemple :

```
## Verse 1
G            D
Pack a little light for the road ahead
Em           C
Leave the porch light on when the sun goes down     ← tu entoures « porch »
```

- **Un rectangle ou un cercle** autour de plusieurs lignes est **ramené à une seule ligne** à la prochaine modification du texte.
- **Une flèche** tracée de droite à gauche se met à **pointer dans l'autre sens**.
- **Un tampon (icône)** **rapetisse** d'environ 40 %. Un petit tampon sous une ligne peut même **disparaître**.
- **Si tu déplaces une annotation**, le serveur garde l'ancien lien. À la modification suivante, elle **revient** près de son ancienne ligne. C'est ce qui est arrivé à ton « Capo ». C'est le cas de 13 de tes 15 annotations liées : elles bougeront toutes à la prochaine modification de leur grille.
- **Corriger une faute de frappe** dans la ligne liée **casse le lien**. L'annotation se fige sans prévenir.
- **Ajouter une copie d'une ligne** au-dessus (un refrain répété) fait **sauter** l'annotation sur la copie.
- **Passer en deux colonnes** ou changer la taille peut couper une longue ligne en deux. Le lien est alors **perdu**.
- **Une annotation dans la marge** n'est jamais liée, et tes icônes de marge restent sur place pendant que le texte bouge.
- **Rien ne s'affiche** dans Studio pour dire ce qui est lié, perdu ou fixe.

Ce qui marche : les surlignages, la transposition (rien ne bouge, comme il faut), et les annotations loin du texte, qui restent fixes.

**En clair : le système actuel déplace des choses au mauvais endroit, sans rien dire.** Il ne faut pas le laisser tourner tel quel, quel que soit ton choix.

## 3. Les possibilités

**A. Ne jamais lier.** Rien ne bouge jamais tout seul.
- **Avantage :** c'est simple et parfaitement prévisible.
- **Inconvénient :** si tu ajoutes une ligne au-dessus de 6 annotations, les 6 sont décalées d'une ligne. Tu dois les redescendre à la main, **sans que rien ne te le signale**, et c'est ce qui partira sur la tablette.

**A′. Ne jamais lier, mais prévenir.** Rien ne bouge, mais après chaque modification du texte, Studio te dit : « 4 annotations ne sont peut-être plus sur leurs mots ». Il les montre, et le rappelle avant la préparation de la tablette. Tu les replaces à la main. C'est un choix honnête, et peu coûteux.

**B. Lier « en bloc » (ma recommandation).** Une annotation est rattachée à **sa ligne**. Quand la ligne monte ou descend, l'annotation **monte ou descend avec elle, d'un seul bloc**. Elle ne change **jamais de forme** : pas d'écrasement, pas de flèche retournée, pas de tampon rapetissé.

Ce que tu peux attendre, cas par cas :

- **Tu ajoutes ou enlèves des lignes, ou une section au-dessus.** L'annotation suit sa ligne, même sur une autre page.
- **Tu corriges une faute** dans la ligne. Elle reste liée. Si tu as ajouté un mot avant le mot entouré, elle se décale avec lui.
- **Tu ajoutes une copie** d'une ligne de refrain. Rien ne change : le serveur sait laquelle est la tienne.
- **Tu changes la taille du texte.** L'annotation grandit ou rétrécit avec le texte, dans les mêmes proportions.
- **Tu passes en deux colonnes.** Elle suit ses mots dans l'autre colonne.
- **Tu supprimes la ligne.** L'annotation **reste où elle était** et elle est marquée **« perdue »**. Studio te le montre. Elle n'est jamais posée en silence sur d'autres mots.
- **Le serveur a un doute** (des lignes ajoutées à l'intérieur d'un grand cadre, par exemple). Elle suit quand même, mais elle est marquée **« à vérifier »**.
- **Tu la déplaces.** Elle se rattache là où tu l'as posée. Loin de tout texte, elle devient **fixe**.
- **Avant la tablette :** la liste des annotations « perdues » ou « à vérifier » s'affiche, parce que la tablette, elle, ne peut rien montrer.

Forme par forme : **c'est la même règle pour toutes.**

| forme | ce qui suit | ce qui ne change jamais |
|---|---|---|
| trait libre | monte et descend avec sa ligne (celle du haut s'il couvre plusieurs lignes) | sa forme : il ne s'étire pas sur les lignes ajoutées, il passe « à vérifier » |
| flèche / trait | idem | sa direction et sa longueur |
| rectangle, cercle, surligneur | idem | sa taille et ses marges |
| texte | idem | sa taille (sauf si la taille de la grille change) |
| tampon, et les deux tampons d'un renvoi | idem, chacun avec sa ligne | sa taille, et le lien du renvoi |
| dans la marge | monte et descend avec la ligne en face | sa position gauche-droite |
| loin de tout texte | rien : fixe | tout |

**Pourquoi B plutôt que A′ :** tes grilles en texte deviennent ton outil principal (taille, colonnes, tablatures). Quand on ajoute une section au-dessus d'une douzaine d'annotations plusieurs fois par semaine, c'est exactement le cas que « ça suit » doit régler. Et tout ce qui est incertain est **montré**, jamais caché.

**Quand préférer A′ :** si tu annotes surtout des PDF importés (c'est le cas aujourd'hui : à peine une vingtaine d'annotations sur des grilles en texte), ou si « rien ne bouge jamais tout seul » compte plus pour toi que les glisser-déposer évités.

## 4. Ce que je propose de faire

1. **Tout de suite :** arrêter le système actuel. Les annotations restent exactement où tu les vois, pour ne plus rien abîmer. En attendant la suite, si tu ajoutes des lignes, les annotations en dessous ne suivent pas.
2. **Ensuite :** construire la version « en bloc ». C'est un vrai chantier : le serveur doit reconnaître une ligne même après une correction.
3. **Puis :** l'affichage dans Studio (lié, à vérifier, perdue, fixe) et la liste avant la tablette.
4. **Enfin :** rattacher tes annotations existantes à leur ligne, **une par une, avec ton accord**, sur une page de revue.

## 5. Les questions pour toi

1. **B** (ça suit en bloc, et les doutes sont signalés) ou **A′** (rien ne bouge, et Studio te dit quoi vérifier) ?
2. Les annotations **dans la marge** suivent-elles la ligne en face ? (Je propose : oui.)
3. Un cadre ou un trait sur plusieurs lignes **ne s'étire jamais**, et il est signalé « à vérifier » quand ces lignes s'écartent ? (Je propose : oui.)
4. Quand la **taille du texte** change, les annotations grandissent ou rétrécissent avec ? (Je propose : oui.)
5. On **arrête le système actuel tout de suite** ? (Je propose : oui.)
6. Les mots à afficher : « lié », « à vérifier », « perdu », « fixe », ou d'autres ?
