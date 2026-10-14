ALTER TABLE user_recommendation_items
    DROP CONSTRAINT IF EXISTS user_recommendation_items_position_check;

ALTER TABLE user_recommendation_items
    ADD CONSTRAINT user_recommendation_items_position_check
    CHECK (position BETWEEN 1 AND 9);
