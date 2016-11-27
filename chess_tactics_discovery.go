//
// chess_tactics_discovery
//
// Usage:
//  $ SQLUSER=root SQLPASS=password SQLIP=127.0.0.1 SQLPORT=3306 ./chess_tactics_discovery -engine=stockfish < test.epd
//
// reads EPD files from standard in and writes discovered blunders (mates, bad moves) to chess_tactics.positions table
// described below (mysql database is called chess_tactics, and has the following table in it):
//
// mysql> desc positions;
// +---------+---------------+------+-----+---------+----------------+
// |Field   | Type          | Null | Key | Default | Extra          |
// +---------+---------------+------+-----+---------+----------------+
// | id      | bigint(20)    | NO   | PRI | NULL    | auto_increment |
// | fen     | varchar(1024) | NO   | MUL | NULL    |                |
// | sm      | varchar(10)   | NO   |     | NULL    |                |
// | cp      | int(11)       | YES  |     | NULL    |                |
// | dm      | int(11)       | YES  |     | NULL    |                |
// | bm      | varchar(10)   | YES  |     | NULL    |                |
// | blunder | int(11)       | YES  |     | NULL    |                |
// +---------+---------------+------+-----+---------+----------------+
// 7 rows in set (0.00 sec)
//
// You will need pgn-extract (http://github.com/atinm/pgn-extract and the db-extract program included here to run:
//
// $ ~/src/pgn-extract/pgn-extract -Wepd -C -N -V -w5000 --nomovenumbers --nochecks --noresults --notags -s ~/src/chess/db/1.pgn | ~/src/pgn-extract/db-extract | SQLUSER=sqluser SQLPASS=sqlpass SQLIP=127.0.0.1 SQLPORT=3306 ~/gouser/bin/chess_tactics_discovery -engine ~/src/Stockfish/src/stockfish
//
package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	_ "github.com/go-sql-driver/mysql"
)

const (
	MAX_CENTIPAWNS = 300
	MAX_MATE_IN = 5
	MOVE_TIME = "1000"
	MAX_DEPTH = "25"
	MIN_MOVES = 12
)

var EngineReader *bufio.Scanner
var EngineIn io.Writer

func send(cmd string, args ...string) (string, string, error) {
	ok := "ok"
	secondary := ""
	
	switch cmd {
	case "uci":
		command := cmd + "\n"
		//log.Print("cmd: ", command)
		_, err := io.WriteString(EngineIn, command)
		if err != nil {
			log.Fatal("Writing %s to engine: %s", command, err.Error())
		}
		
		// read until we see "uciok"
		for EngineReader.Scan() {
			//log.Println(EngineReader.Text())
			if EngineReader.Text() == "uciok" {
				break
			}
		}
		
	case "position":
		command := "position fen " + args[0] + "\n"
		//log.Print("cmd: ", command)
		_, err := io.WriteString(EngineIn, command)
		if err != nil {
			log.Fatal("Writing %s to engine: %s", command, err.Error())
		}
		
	case "go":		
		command := "go"
		for _, arg := range args {
			command = command + " " + arg
		}
		command += "\n"
		//log.Print("cmd: ", command)
		_, err := io.WriteString(EngineIn, command)
		if err != nil {
			log.Fatal("Writing %s to engine: %s", command, err.Error())
		}

		// read until we see "bestmove"
		for EngineReader.Scan() {
			//log.Println(EngineReader.Text())
			if strings.HasPrefix(EngineReader.Text(), "bestmove") {
				rebm := regexp.MustCompile("bestmove ([a-z0-9]+)")
				bmarr := rebm.FindStringSubmatch(EngineReader.Text())
				if len(bmarr) > 1 {
					ok = bmarr[1]
				}
				break
			}
			if strings.HasPrefix(EngineReader.Text(), "info") {
				secondary = EngineReader.Text()
			}
		}
		
	default:
		return "error", "", errors.New("Unrecognized cmd: " + cmd)
	}
	
	if err := EngineReader.Err(); err != nil {
		log.Fatal("Reading engine output: ", err)
		return err.Error(), "", err
	}
	return ok, secondary, nil
}

func eval(fen string, move string) (string, int, int, error) {
	recp := regexp.MustCompile(" cp (-?[0-9]+) ")
	redm := regexp.MustCompile(" mate (-?[0-9]+) ")
	bm := move
	cp := 0
	dm := 0
	
	_, _, err := send("position", fen)
	if err != nil {
		log.Fatal("Error: %s", err.Error())
		return "", 0, 0, err
	}
	info := ""
	if len(move) == 0 {
		// find best move
		bm, info, err = send("go", "movetime", MOVE_TIME)
	} else {
		// find cp, dm for move
		bm, info, err = send("go", "movetime", MOVE_TIME, "searchmoves", move)
	}
	
	if err != nil {
		log.Fatal("Error: %s", err.Error())
		return "", 0, 0, err
	} else {
		cparr := recp.FindStringSubmatch(info)
		if len(cparr) > 1 {
			cp, err = strconv.Atoi(cparr[1])
			if err != nil {
				log.Fatal("Error: %s", err.Error())
				return "", 0, 0, err
			}
		}
		dmarr := redm.FindStringSubmatch(info)
		if len(dmarr) > 1 {
			dm, err = strconv.Atoi(dmarr[1])
			if err != nil {
				log.Fatal("Error: %s", err.Error())
				return "", 0, 0, err
			}
		}
	}

	return bm, cp, dm, nil
}

func main() {
	var err error
	engine := flag.String("engine", "stockfish", "Chess engine full path")
	flag.Parse()
	
	// start chess engine
	log.Println("Starting engine: ", *engine)
	
	cmd := exec.Command(*engine)
	
	cmd.Stderr = os.Stderr
	EngineIn, err = cmd.StdinPipe()
	if nil != err {
		log.Fatalf("Error obtaining stdin: %s", err.Error())
	}
	engineOut, err := cmd.StdoutPipe()
	if nil != err {
		log.Fatalf("Error obtaining stdout: %s", err.Error())
	}
	EngineReader = bufio.NewScanner(engineOut)
	
	if cmd.Start() != nil {
		log.Fatal(err)
	}
	defer cmd.Process.Kill()

	// read engine hello
	EngineReader.Scan()
	log.Println(EngineReader.Text())
	
	send("uci")

	sqlstr := os.ExpandEnv("${SQLUSER}:${SQLPASS}@tcp(${SQLIP}:${SQLPORT})/chess_tactics")
	db, err := sql.Open("mysql", sqlstr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	stmt, err := db.Prepare("INSERT INTO positions(fen, sm, cp, dm, bm, blunder) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	
	stdin := bufio.NewReader(os.Stdin)
	r := csv.NewReader(stdin)
	white := true
	prevwcp, prevbcp, prevcp := 0, 0, 0
	games := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(record) < 3 {
			log.Fatal("Records have ", len(record), " items.", record)
		}
		move_num, err := strconv.Atoi(record[0])
		if err != nil {
			log.Fatal(err)
		} else {
			if move_num < MIN_MOVES {
				continue
			}
		}
		
		fen := record[1]
		sm := record[2]
		blunder := 0
		
		// run evaluation of sm
		_, smcp, smdm, err := eval(fen, sm)
		if err != nil {
			log.Fatal(err.Error())
		}

		if move_num == MIN_MOVES {
			if white {
				prevwcp = smcp
			} else {
				prevbcp = smcp
			}
			// flip move color
			white = !white
			
			if white {
				// start counting
				games += 1
				fmt.Printf("Games: %d\r", games)
			}
			continue
		}
		
		blunder = 0
		if white {
			prevcp = prevwcp
			prevwcp = smcp
		} else {
			prevcp = prevbcp
			prevbcp = smcp
		}
		
		if smdm < 0 {
			// look for mate
			if smdm >= -MAX_MATE_IN {
				// move results in checkmate in MAX_MATE_IN
				blunder = 10000
			}
		} else if smcp < 0 && smcp < prevcp && prevcp - smcp >= MAX_CENTIPAWNS {
			// look for bad move by centipawns
			blunder = prevcp - smcp
		}


		if blunder > 0 {
			// run evaluation for best move
			bm, bmcp,bmdm, err := eval(fen, "")
			if err != nil {
				log.Fatal(err.Error())
			}

			if bm != sm && ((bmcp > 0 && bmcp - smcp >= MAX_CENTIPAWNS) || (bmdm > 0 && bmdm < MAX_MATE_IN)) {
				log.Println("Inserting ", fen, sm, smcp, smdm, bm, blunder, " into database")
				
				res, err := stmt.Exec(fen, sm, smcp, smdm, bm, blunder)
				if err != nil {
					// possible duplicate
					//log.Println(err)
					continue
				}
				lastId, err := res.LastInsertId()
				if err != nil {
					log.Fatal(err)
				}
				rowCnt, err := res.RowsAffected()
				if err != nil {
					log.Fatal(err)
				}
				
				log.Printf("ID = %d, affected = %d\n", lastId, rowCnt)
			}
		}

		// flip move color
		white = !white
	}
}

