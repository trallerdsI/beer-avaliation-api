import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';

abstract interface class IUserPreferencesRepository {
  Future<Either<Failure, List<int>>> getFavoriteIds();

  Future<Either<Failure, Unit>> toggleFavorite(int beerId);

  Future<Either<Failure, int?>> getUserRatingForBeer(int beerId);

  Future<Either<Failure, Unit>> setUserRating(int beerId, int stars);

  Future<Either<Failure, Unit>> removeUserRating(int beerId);

  Future<Either<Failure, List<int>>> getLikedCommentIds();

  Future<Either<Failure, Unit>> toggleCommentLike(int commentId);

  Future<Either<Failure, String?>> getSavedSearchQuery();

  Future<Either<Failure, Unit>> setSavedSearchQuery(String query);

  Future<Either<Failure, String?>> getSavedStyleFilter();

  Future<Either<Failure, Unit>> setSavedStyleFilter(String? filter);
}
