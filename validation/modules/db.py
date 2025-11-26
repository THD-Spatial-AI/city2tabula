from sqlalchemy import create_engine, text
from sqlalchemy.exc import SQLAlchemyError


def get_db_engine(config):
    """
    Create and return a SQLAlchemy engine based on the config dictionary.
    Returns:
        engine (sqlalchemy.Engine) or None
    """
    conn_str = config["db"].get("connection_string")

    if not conn_str:
        raise ValueError("Missing 'connection_string' in config['db'].")

    try:
        engine = create_engine(conn_str)

        # Test connection
        with engine.connect() as connection:
            connection.execute(text("SELECT 1"))

        return engine

    except SQLAlchemyError as e:
        raise RuntimeError(f"Failed to connect to database: {e}") from e


def close_db_engine(engine):
    """Dispose of the SQLAlchemy database engine."""
    if engine:
        engine.dispose()

