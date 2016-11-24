Usage:
```
 $ SQLUSER=root SQLPASS=password SQLIP=127.0.0.1 SQLPORT=3306 ./chess_tactics_discovery -engine=stockfish < test.epd
```
reads EPD files from standard in and writes discovered blunders (mates, bad moves) to chess_tactics.positions table
described below (mysql database is called chess_tactics, and has the following table in it):

```
mysql> desc positions;
+---------+---------------+------+-----+---------+----------------+
|Field   | Type          | Null | Key | Default | Extra          |
+---------+---------------+------+-----+---------+----------------+
| id      | bigint(20)    | NO   | PRI | NULL    | auto_increment |
| fen     | varchar(1024) | NO   | MUL | NULL    |                |
| sm      | varchar(10)   | NO   |     | NULL    |                |
| cp      | int(11)       | YES  |     | NULL    |                |
| dm      | int(11)       | YES  |     | NULL    |                |
| bm      | varchar(10)   | YES  |     | NULL    |                |
| blunder | int(11)       | YES  |     | NULL    |                |
+---------+---------------+------+-----+---------+----------------+
7 rows in set (0.00 sec)
```
You will need pgn-extract (http://github.com/atinm/pgn-extract and the db-extract program included here to run:
```
$ ~/src/pgn-extract/pgn-extract -Wepd -C -N -V -w5000 --nomovenumbers --nochecks --noresults --notags -s ~/src/chess/db/1.pgn | ~/src/pgn-extract/db-extract | SQLUSER=sqluser SQLPASS=sqlpass SQLIP=127.0.0.1 SQLPORT=3306 ~/gouser/bin/chess_tactics_discovery -engine ~/src/Stockfish/src/stockfish
```

