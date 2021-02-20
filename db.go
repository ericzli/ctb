package main

import (
	"database/sql"
	"fmt"
)

var s_DB *sql.DB

func initDb(uri string) {
	if s_DB != nil {
		return
	}
	DB, err := sql.Open("mysql", uri)
	if err != nil || DB == nil {
		panic(fmt.Sprintf("Connect mysql failed, %v", err))
	}
	s_DB = DB
}

/*
CREATE TABLE `ctb_choice_question` (
	`id` int(11) NOT NULL AUTO_INCREMENT,
	`type` bigint(20) NOT NULL,
	`question` varchar(256) NOT NULL,
	`right_answer` varchar(256) NOT NULL,
	`wrong_answer` varchar(256) NOT NULL,
	PRIMARY KEY (`id`)
) ENGINE=MyISAM AUTO_INCREMENT=8 DEFAULT CHARSET=utf8;

CREATE TABLE `ctb_user` (
	`id` int(11) NOT NULL AUTO_INCREMENT,
	`user` char(64) NOT NULL,
	PRIMARY KEY (`id`)
) ENGINE=MyISAM DEFAULT CHARSET=utf8;

CREATE TABLE `ctb_answer_record` (
	`question_id` int(11) NOT NULL,
	`user_id`  int(11) NOT NULL,
	`rest_cnt` int(11) NOT NULL,
	`next_time` datetime NOT NULL,
	`right_cnt` int(11) DEFAULT '0',
	`wrong_cnt` int(11) DEFAULT '0',
	PRIMARY KEY (`question_id`,`user_id`)
) ENGINE=MyISAM DEFAULT CHARSET=utf8
partition by hash(user_id) partitions 3;
*/
