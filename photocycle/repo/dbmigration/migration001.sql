SET NAMES 'utf8';
SET SESSION sql_mode='NO_AUTO_VALUE_ON_ZERO';

INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(-330, 'Ошибка zip', 0, 0, 0, 0);
INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(118, 'Распаковка zip', 0, 0, 0, 0);
INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(119, 'Преобразование PP', 0, 0, 0, 0);

INSERT INTO src_type(id, loc_type, name, state, book_part) VALUES(25, 1, 'PixelPark', 0, 0);