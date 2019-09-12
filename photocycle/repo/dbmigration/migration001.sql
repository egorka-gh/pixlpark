SET NAMES 'utf8';
SET SESSION sql_mode='NO_AUTO_VALUE_ON_ZERO';

INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(-330, 'Ошибка zip', 0, 0, 0, 0);
INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(118, 'Распаковка zip', 0, 0, 0, 0);
INSERT INTO order_state(id, name, runtime, extra, tech, book_part) VALUES(119, 'Преобразование PP', 0, 0, 0, 0);

INSERT INTO src_type(id, loc_type, name, state, book_part) VALUES(25, 1, 'PixelPark', 0, 0);

--2019-08-30 applied on main cycle

-- hpotoprint formats
/*
SELECT AT.id, AT.name, AT.field, av.id avid, av.value, s.synonym
  FROM attr_type at
    INNER JOIN attr_value av ON at.id = av.attr_tp
    INNER JOIN attr_synonym s ON av.id = s.attr_val AND s.src_type = 4
  WHERE AT.id IN (11, 12)
  ORDER BY s.synonym, AT.id
*/


DELIMITER $$

CREATE 
PROCEDURE pp_StartOrders (IN pSourse int, IN pGroupId int, IN pExcludeId varchar(50))
BEGIN
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    ROLLBACK;
  END;

  START TRANSACTION;
    -- book and other (all by alias)
    -- StateLoadComplite -> StatePreprocessWaite
    UPDATE orders o
      SET o.state = 150
      WHERE o.source = pSourse AND o.group_id = pGroupId AND o.id != pExcludeId AND o.state = 130;

    -- photo print StatePreprocessComplite -> StatePrintWaite
    -- print groups
    UPDATE print_group pg
    INNER JOIN orders o
      ON o.id = pg.order_id
      SET pg.state = 200
      WHERE o.source = pSourse AND o.group_id = pGroupId AND o.id != pExcludeId AND o.state = 180;
    -- orders
    UPDATE orders o
      SET o.state = 200
      WHERE o.source = pSourse AND o.group_id = pGroupId AND o.id != pExcludeId AND o.state = 180;
  COMMIT;
END
$$

DELIMITER ;